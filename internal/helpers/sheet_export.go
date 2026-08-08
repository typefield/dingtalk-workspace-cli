package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/spf13/cobra"
)

func runSheetExport(cmd *cobra.Command, _ []string) error {
	nodeID := mustGetFlag(cmd, "node")
	if nodeID == "" {
		return apperrors.NewValidation("flag --node is required", apperrors.WithSubtype(apperrors.SubtypeMissingRequiredFlags))
	}
	outputPath, _ := cmd.Flags().GetString("output")

	if deps.Caller.DryRun() {
		return writeCommandPayload(cmd, map[string]any{
			"executed":     false,
			"preview_kind": "plan",
			"operation":    "export_sheet_xlsx",
			"node_id":      nodeID,
			"output":       outputPath,
		})
	}

	ctx := context.Background()

	// JSON 模式下进度只允许走 stderr，stdout 由统一返回在命令终点一次写出。
	jsonMode := deps.Caller.Format() == "json"

	// Step 1: submit export job
	if !jsonMode {
		deps.Out.PrintInfo("[1/3] 提交表格导出任务 (xlsx)...")
	}
	submitText, err := callMCPToolReturnText(ctx, "submit_export_job", map[string]any{
		"nodeId":       nodeID,
		"exportFormat": "xlsx",
	})
	if err != nil {
		return fmt.Errorf("提交导出任务失败: %w", err)
	}
	jobID, err := parseExportSubmitResult(submitText)
	if err != nil {
		return err
	}
	if !jsonMode {
		deps.Out.PrintInfo(fmt.Sprintf("导出任务已提交: jobId=%s", jobID))
		// Step 2: progressive backoff polling
		deps.Out.PrintInfo("[2/3] 轮询任务状态（渐进式退避，最多 30 次约 5 分钟）...")
	}
	downloadURL, err := pollSheetExportJob(ctx, jobID)
	if err != nil {
		return err
	}

	// No output path: return the expiring download URL without downloading.
	if outputPath == "" {
		return writeCommandPayload(cmd, map[string]any{
			"job_id":       jobID,
			"download_url": downloadURL,
		})
	}

	// Step 3: download to local file
	// If outputPath is an existing directory, append inferred filename.
	if fi, statErr := os.Stat(outputPath); statErr == nil && fi.IsDir() {
		filename := inferSheetExportFilename(downloadURL)
		if filename == "" {
			filename = fmt.Sprintf("sheet-export-%s.xlsx", jobID)
		}
		outputPath = filepath.Join(outputPath, filename)
	}

	if !jsonMode {
		deps.Out.PrintInfo(fmt.Sprintf("[3/3] 下载 xlsx 到 %s ...", outputPath))
	}
	if err := httpGetFile(ctx, downloadURL, map[string]string{}, outputPath); err != nil {
		return fmt.Errorf("下载 xlsx 失败: %w", err)
	}
	return writeCommandPayload(cmd, map[string]any{
		"job_id":       jobID,
		"output_path":  outputPath,
		"download_url": downloadURL,
	})
}

// parseExportSubmitResult extracts jobId from submit_export_job MCP response.
func parseExportSubmitResult(text string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return "", fmt.Errorf("解析 submit_export_job 响应失败: %w", err)
	}
	if result, ok := data["result"].(map[string]any); ok {
		data = result
	}
	if success, ok := data["success"].(bool); ok && !success {
		msg, _ := data["message"].(string)
		if msg == "" {
			msg = "提交导出任务失败"
		}
		return "", fmt.Errorf("%s", msg)
	}
	jobID, _ := data["jobId"].(string)
	if jobID == "" {
		return "", fmt.Errorf("submit_export_job 未返回 jobId，响应: %s", text)
	}
	return jobID, nil
}

// exportPollIntervals returns the progressive backoff schedule defined in the
// sheet export MCP tool spec: 1~5:2s, 6~10:5s, 11~20:10s, 21~30:15s.
func exportPollIntervals() []time.Duration {
	intervals := make([]time.Duration, 0, 30)
	for i := 0; i < 5; i++ {
		intervals = append(intervals, 2*time.Second)
	}
	for i := 0; i < 5; i++ {
		intervals = append(intervals, 5*time.Second)
	}
	for i := 0; i < 10; i++ {
		intervals = append(intervals, 10*time.Second)
	}
	for i := 0; i < 10; i++ {
		intervals = append(intervals, 15*time.Second)
	}
	return intervals
}

// pollExportJob polls query_export_job per the progressive backoff schedule
// until the job completes successfully, fails, or the 30-attempt cap is hit.
func pollSheetExportJob(ctx context.Context, jobID string) (string, error) {
	// json 模式下轮询进度也要抑制，否则 [INFO] 行会混进 stdout 破坏纯 JSON 输出。
	quiet := deps.Caller.Format() == "json"
	intervals := exportPollIntervals()
	for i, wait := range intervals {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-helperAfter(wait):
		}

		text, err := callMCPToolReturnText(ctx, "query_export_job", map[string]any{
			"jobId": jobID,
		})
		if err != nil {
			if !quiet {
				deps.Out.PrintInfo(fmt.Sprintf("  [%d/30] 查询失败，将继续轮询: %v", i+1, err))
			}
			continue
		}

		status, downloadURL, message, parseErr := parseExportQueryResult(text)
		if parseErr != nil {
			return "", parseErr
		}

		// 服务端可能返回 SUCCESS / success / Success 等不同大小写，统一归一化后再比较。
		normStatus := strings.ToUpper(strings.TrimSpace(status))
		switch normStatus {
		case "SUCCESS":
			if !quiet {
				deps.Out.PrintInfo(fmt.Sprintf("  [%d/30] 状态: SUCCESS", i+1))
			}
			if downloadURL == "" {
				return "", fmt.Errorf("任务成功但未返回 downloadUrl")
			}
			return downloadURL, nil
		case "FAILED", "FAIL", "ERROR":
			if message == "" {
				message = "导出任务失败"
			}
			return "", fmt.Errorf("%s", message)
		case "PROCESSING", "RUNNING", "DOING", "PENDING", "":
			if !quiet {
				deps.Out.PrintInfo(fmt.Sprintf("  [%d/30] 状态: PROCESSING", i+1))
			}
		default:
			if !quiet {
				deps.Out.PrintInfo(fmt.Sprintf("  [%d/30] 状态: %s", i+1, status))
			}
		}
	}
	return "", fmt.Errorf("导出任务超时：已轮询 30 次（约 5 分钟）仍未完成，请稍后再试")
}

// parseExportQueryResult extracts status/downloadUrl/message from query_export_job.
func parseExportQueryResult(text string) (status, downloadURL, message string, err error) {
	var data map[string]any
	if e := json.Unmarshal([]byte(text), &data); e != nil {
		err = fmt.Errorf("解析 query_export_job 响应失败: %w", e)
		return
	}
	if result, ok := data["result"].(map[string]any); ok {
		data = result
	}
	status, _ = data["status"].(string)
	downloadURL, _ = data["downloadUrl"].(string)
	message, _ = data["message"].(string)
	return
}

// inferSheetExportFilename extracts a safe local filename from a sheet-export download URL.
func inferSheetExportFilename(rawURL string) string {
	name := ""
	if idx := strings.LastIndex(rawURL, "/"); idx >= 0 && idx < len(rawURL)-1 {
		name = rawURL[idx+1:]
		if qIdx := strings.Index(name, "?"); qIdx >= 0 {
			name = name[:qIdx]
		}
	}
	if name == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(name); err == nil && decoded != "" {
		name = decoded
	}
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "" || name == "." || name == "/" {
		return ""
	}
	return name
}

// ── export 命令定义 ──────────────────────────────────────────────────────────

func newExportCmd() *cobra.Command {
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "导出表格为 xlsx（异步任务一站式）",
		Long: `将钉钉在线电子表格导出为 Office xlsx 格式（单命令一站式）。

执行流程（全程自动，无需 Agent 介入轮询）:
  1. 提交导出任务（submit_export_job），获取 jobId
  2. 按渐进式退避策略轮询任务状态（query_export_job）
       第 1~5 次：每次 2 秒
       第 6~10 次：每次 5 秒
       第 11~20 次：每次 10 秒
       第 21~30 次：每次 15 秒
       硬上限 30 次（约 5 分钟），超时后返回错误
  3. 任务成功后取得 downloadUrl
  4. 若指定了 --output，将 xlsx 下载到本地文件；否则直接输出 downloadUrl

参数说明:
  --node    表格文档 ID 或链接 URL，系统自动识别（必填）
  --output  本地保存路径（可选）。可为文件路径或目录：
            - 文件路径：如 ./a.xlsx，直接按此路径保存
            - 目录路径：如 ./，自动从下载链接推断文件名
            - 未指定：仅返回 downloadUrl，链接有时效性请尽快下载

支持范围:
  仅支持钉钉在线电子表格（axls）→ xlsx；
  若需导出钉钉文字文档，请使用 dingtalkdoc 侧的导出工具。

权限要求:
  当前用户对目标表格具备可查看/下载权限。`,
		Example: `  # 仅导出，返回 downloadUrl（链接有时效性，请尽快下载）
  dws sheet export --node NODE_ID

  # 导出并自动下载为本地文件
  dws sheet export --node NODE_ID --output ./report.xlsx

  # --output 为目录时，自动按下载链接里的文件名保存
  dws sheet export --node "https://alidocs.dingtalk.com/i/nodes/<DOC_UUID>" --output ./`,
		RunE: runSheetExport,
	}
	exportCmd.Flags().String("node", "", "表格文档 ID 或 URL (必填)")
	exportCmd.Flags().String("output", "", "本地保存路径（可选，支持文件路径或目录）")
	return exportCmd
}
