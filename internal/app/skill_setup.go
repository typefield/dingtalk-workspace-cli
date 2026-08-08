package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// skillSetupAgentHomes is the ordered list of agent home subdirectories
// where dws skills get installed. Mirrors install.sh / install.ps1 /
// build/npm/install.js so that `dws skill setup` and the install scripts
// agree on the install footprint.
var skillSetupAgentHomes = []string{
	".agents/skills",
	".claude/skills",
	".cursor/skills",
	".qoder/skills",
	".qoderwork/skills",
	".gemini/skills",
	".codex/skills",
	".github/skills",
	".windsurf/skills",
	".augment/skills",
	".cline/skills",
	".amp/skills",
	".kiro/skills",
	".trae/skills",
	".openclaw/skills",
	".hermes/skills",
}

const (
	skillSetupModeMono  = "mono"
	skillSetupModeMulti = "multi"
)

var (
	skillSetupResolveMode    = resolveSkillSetupMode
	skillSetupResolveSource  = resolveSkillSetupSourceOrEmbedded
	skillSetupResolveTargets = resolveSkillSetupTargets
	skillSetupListMulti      = listMultiSkillNames
	skillSetupFilterMulti    = filterMultiSkillNames
	skillSetupApply          = applySkillSetup
	skillSetupCopyDir        = copyDir
	skillSetupRunForm        = (*huh.Form).Run
	skillSetupInteractive    = isInteractiveTerminal
	skillSetupReadDir        = os.ReadDir
	skillSetupStat           = os.Stat
	skillSetupExecutable     = os.Executable
	skillSetupGetwd          = os.Getwd
	skillSetupUserHomeDir    = os.UserHomeDir
	skillSetupRemoveAll      = os.RemoveAll
	skillSetupMkdirAll       = os.MkdirAll
	skillSetupWalk           = filepath.Walk
	skillSetupRel            = filepath.Rel
	skillSetupReadlink       = os.Readlink
	skillSetupOpen           = os.Open
	skillSetupOpenFile       = os.OpenFile
	skillSetupCopy           = io.Copy
)

func newSkillSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "安装 dws 自身 skill 到 Agent 目录",
		Long: `安装 dws 自身 skill 文档到 AI Agent 目录（如 ~/.claude/skills/、~/.cursor/skills/ 等）。

支持两种模式：
  mono   单 skill（稳定 / 推荐）—— 总入口 SKILL.md + references/products/
  multi  多 skill—— 按产品拆 N 个独立 skill

multi 模式支持按产品挑选：
  -s/--skill   只装指定子 skill（可重复，短名 aitable 或全名 dingtalk-aitable 均可）
  -x/--exclude 从全装里剔除指定子 skill（可重复，与 --skill 互斥）
  未列出的已有 dingtalk-* skill 会保留（additive 叠加语义）

不带 --mode 时进入交互式询问；不带 --target 时铺到所有检测到的 Agent 目录。
skill 源默认取二进制内嵌的版本（升级二进制即升级 skill）；--source / DWS_SKILL_SOURCE 可显式覆盖。`,
		Example: `  dws skill setup                                       # 交互式
  dws skill setup --mode mono --yes                     # 非交互装 mono
  dws skill setup --mode multi --target claude          # multi 全装到 ~/.claude/skills/
  dws skill setup --mode multi -s aitable -s calendar   # 只装 aitable + calendar
  dws skill setup --mode multi -x live -x devdoc        # 安装除 live、devdoc 外的其余 skill
  dws skill setup --source /path/to/repo                # 显式指定 skill 源`,
		DisableAutoGenTag: true,
		RunE:              runSkillSetup,
	}
	cmd.Flags().String("mode", "", "skill 模式：mono | multi（不指定则交互询问）")
	cmd.Flags().String("target", "all", "目标 Agent：all | "+skillSetupSupportedTargets())
	cmd.Flags().String("source", "", "skill 源目录（默认使用二进制内嵌的 skill 源，与当前版本一致）")
	cmd.Flags().Bool("yes", false, "跳过所有确认提示")
	cmd.Flags().StringSliceP("skill", "s", nil, "multi 模式：仅安装指定子 skill（可重复，接受短名 aitable 或全名 dingtalk-aitable）")
	cmd.Flags().StringSliceP("exclude", "x", nil, "multi 模式：从全装中剔除指定子 skill（可重复，与 --skill 互斥）")
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		OutputRollout: output.RolloutV2Active,
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "high",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Validate: validateSkillSetup,
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "skill",
				Name:           "setup",
				CanonicalPath:  "skill.setup",
				CLIPath:        "skill setup",
				PrimaryCLIPath: "skill setup",
			},
			Description: "把当前 dws 版本携带的 mono 或 multi Skill 安装到一个或多个本地 Agent 目录，并清理互斥模式残留",
			DryRun:      &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewPlan, RemoteReads: false},
			Result: &contract.ResultSpec{
				Outcomes: []contract.ResultOutcome{
					contract.ResultOutcomeSuccess,
					contract.ResultOutcomePartialFailure,
					contract.ResultOutcomeFailure,
				},
				DataSchema: json.RawMessage(`{"oneOf":[{"type":"object","properties":{"mode":{"type":"string","enum":["mono","multi"]},"source":{"type":"string"},"preview_kind":{"type":"string"},"targets":{"type":"array","items":{"type":"string"}},"skills":{"type":"array","items":{"type":"string"}},"removals":{"type":"array","items":{"type":"string"}},"installs":{"type":"array","items":{"type":"string"}},"installed":{"type":"integer","minimum":0},"removed":{"type":"integer","minimum":0},"operations":{"type":"array","items":{"type":"object"}}},"required":["mode","source","targets"],"additionalProperties":false},{"type":"object","properties":{"total":{"type":"integer","minimum":1},"succeeded":{"type":"array","minItems":1,"items":{"type":"object"}},"failed":{"type":"array","items":{"type":"object"}},"unknown":{"type":"array","items":{"type":"object"}}},"required":["total","succeeded","failed","unknown"],"additionalProperties":false}]}`),
			},
			Interface: &contract.InterfaceSpec{
				Mode:         contract.InterfaceModeLocal,
				Availability: contract.InterfaceAvailable,
				Reason:       "命令只读取二进制内嵌或用户显式指定的本地 Skill 源，并修改本机 Agent 技能目录，不调用远端接口",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "预览或安装当前 dws 版本携带的 Agent Skill 到本机 Agent 目录",
				UseWhen:      []string{"用户明确要求为本机 Agent 安装或切换 dws mono/multi Skill"},
				AvoidWhen: []string{
					"只需要从技能市场安装单个已知 skillId 时使用 skill install",
					"用户尚未确认将被覆盖或删除的本地 Skill 目录时不要正式执行",
				},
				Examples: []string{"dws skill setup --mode mono --target codex --dry-run --format json"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "mode", Description: "安装模式：mono 或 multi"},
				{Name: "target", Description: "目标 Agent 名称或 all"},
				{Name: "source", Description: "可选本地 Skill 源覆盖目录"},
				{Name: "skill", Description: "multi 模式下只安装这些子 Skill"},
				{Name: "exclude", Description: "multi 模式下排除这些子 Skill"},
			},
		},
	})
	return cmd
}

// skillSetupSupportedTargets deliberately excludes ".": setup manages
// registered Agent homes and may delete opposite-mode siblings. Current-dir
// installation remains available only on the single-package `skill install`
// command, whose footprint does not perform that mutual-exclusion cleanup.
func skillSetupSupportedTargets() string {
	targets := make([]string, 0, len(agentSkillPaths))
	for target := range agentSkillPaths {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return strings.Join(targets, ", ")
}

func validateSkillSetup(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return apperrors.NewValidation("skill setup does not accept positional arguments")
	}
	mode, _ := cmd.Flags().GetString("mode")
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "" && mode != skillSetupModeMono && mode != skillSetupModeMulti {
		return apperrors.NewValidation(fmt.Sprintf("不支持的 --mode 值: %s（可选 mono / multi）", mode))
	}
	target, _ := cmd.Flags().GetString("target")
	target = strings.ToLower(strings.TrimSpace(target))
	if target != "" && target != "all" {
		if _, ok := agentSkillPaths[target]; !ok {
			return apperrors.NewValidation(fmt.Sprintf("不支持的 --target 值: %s（可选 all, %s）", target, skillSetupSupportedTargets()))
		}
	}
	include, _ := cmd.Flags().GetStringSlice("skill")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	if len(include) > 0 && len(exclude) > 0 {
		return apperrors.NewValidation("--skill 与 --exclude 不能同时使用")
	}
	if mode == skillSetupModeMono && (len(include) > 0 || len(exclude) > 0) {
		return apperrors.NewValidation("--skill / --exclude 仅在 --mode multi 下有效（mono 只有一个 skill，无需挑选）")
	}
	if mode == "" && (len(include) > 0 || len(exclude) > 0) {
		return apperrors.NewValidation("使用 --skill / --exclude 时必须显式指定 --mode multi")
	}
	return nil
}

func runSkillSetup(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	target, _ := cmd.Flags().GetString("target")
	source, _ := cmd.Flags().GetString("source")
	autoYes := corecmd.BoolFlag(cmd, "yes")
	includeRaw, _ := cmd.Flags().GetStringSlice("skill")
	excludeRaw, _ := cmd.Flags().GetStringSlice("exclude")
	dryRun := corecmd.BoolFlag(cmd, "dry-run")

	// Mode selection diagnostics belong on stderr. stdout is reserved for the
	// one framework result document once this command is v2-active.
	mode, err := skillSetupResolveMode(mode, autoYes, cmd.ErrOrStderr())
	if err != nil {
		return apperrors.NewValidation(err.Error())
	}

	if mode == skillSetupModeMono && (len(includeRaw) > 0 || len(excludeRaw) > 0) {
		return apperrors.NewValidation("--skill / --exclude 仅在 --mode multi 下有效（mono 只有一个 skill，无需挑选）")
	}

	dests, err := skillSetupResolveTargets(target, mode)
	if err != nil {
		return apperrors.NewValidation(err.Error())
	}

	// Dry-run must not materialize the embedded bundle into a temporary
	// directory. Inspect either the explicit local source or embed.FS directly,
	// then return the complete deletion/install plan as the sole stdout result.
	if dryRun {
		preview, previewErr := inspectSkillSetupSource(source, mode)
		if previewErr != nil {
			if strings.TrimSpace(source) != "" || strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")) != "" {
				return apperrors.NewValidation(previewErr.Error())
			}
			return apperrors.NewInternal(previewErr.Error())
		}
		multiNames, filterErr := selectSkillSetupMultiNames(mode, preview.MultiSkillNames, includeRaw, excludeRaw)
		if filterErr != nil {
			return apperrors.NewValidation(filterErr.Error())
		}
		return output.StoreResult(cmd.Context(), output.Success(
			buildSkillSetupPlan(mode, preview.Label, dests, multiNames),
			output.WithDryRun(),
		))
	}

	skillSrc, srcCleanup, err := skillSetupResolveSource(source, mode)
	if err != nil {
		if strings.TrimSpace(source) != "" || strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")) != "" {
			return apperrors.NewValidation(err.Error())
		}
		return apperrors.NewInternal(err.Error())
	}
	defer srcCleanup()
	// Keep the result contract stable and user-facing. The temporary directory
	// used to execute an embedded bundle is an implementation detail and is
	// removed before the command returns; never expose that ephemeral path as
	// the installed source.
	sourceLabel := skillSrc
	if strings.TrimSpace(source) == "" && strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")) == "" {
		sourceLabel = "embedded://skills/" + mode
	}

	var allMultiSkillNames []string
	if mode == skillSetupModeMulti {
		allMultiSkillNames, err = skillSetupListMulti(skillSrc)
		if err != nil {
			return apperrors.NewInternal(err.Error())
		}
	}
	multiSkillNames, err := selectSkillSetupMultiNames(mode, allMultiSkillNames, includeRaw, excludeRaw)
	if err != nil {
		return apperrors.NewValidation(err.Error())
	}

	report := skillSetupApply(mode, skillSrc, multiSkillNames, dests)
	return output.StoreResult(cmd.Context(), skillSetupResult(mode, sourceLabel, dests, multiSkillNames, report))
}

func selectSkillSetupMultiNames(mode string, all, include, exclude []string) ([]string, error) {
	if mode != skillSetupModeMulti {
		return []string{}, nil
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("multi 模式下未发现含 SKILL.md 的子目录")
	}
	filtered, err := skillSetupFilterMulti(all, include, exclude)
	if err != nil {
		return nil, err
	}
	// dingtalk-shared carries the global rules every product skill declares as
	// a PREREQUISITE and must ship with every narrowed multi selection.
	return ensureMandatorySharedSkill(filtered, all), nil
}

type skillSetupPlan struct {
	PreviewKind string   `json:"preview_kind"`
	Mode        string   `json:"mode"`
	Source      string   `json:"source"`
	Targets     []string `json:"targets"`
	Skills      []string `json:"skills"`
	Removals    []string `json:"removals"`
	Installs    []string `json:"installs"`
}

func buildSkillSetupPlan(mode, source string, dests, skillNames []string) skillSetupPlan {
	targets := append([]string(nil), dests...)
	sort.Strings(targets)
	skills := append([]string(nil), skillNames...)
	if skills == nil {
		skills = []string{}
	}
	plan := skillSetupPlan{
		PreviewKind: contract.DryRunPreviewPlan,
		Mode:        mode,
		Source:      source,
		Targets:     targets,
		Skills:      skills,
		Removals:    []string{},
		Installs:    []string{},
	}
	for _, dest := range targets {
		plan.Removals = append(plan.Removals, mutualExclusionVictims(dest, mode)...)
		switch mode {
		case skillSetupModeMono:
			plan.Removals = append(plan.Removals, dest)
			plan.Installs = append(plan.Installs, dest)
		case skillSetupModeMulti:
			for _, name := range skills {
				path := filepath.Join(dest, name)
				plan.Removals = append(plan.Removals, path)
				plan.Installs = append(plan.Installs, path)
			}
		}
	}
	sort.Strings(plan.Removals)
	plan.Removals = uniqueSkillSetupPaths(plan.Removals)
	sort.Strings(plan.Installs)
	return plan
}

func uniqueSkillSetupPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}
	out := paths[:0]
	for _, path := range paths {
		if len(out) == 0 || out[len(out)-1] != path {
			out = append(out, path)
		}
	}
	return out
}

// multiSkillPrefix is the canonical prefix for every per-product skill
// bundle in skills/multi/ (e.g. dingtalk-aitable, dingtalk-calendar).
const multiSkillPrefix = "dingtalk-"

// multiSharedSkill is the shared, non-product skill that every per-product
// skill declares as a PREREQUISITE. It must always be installed in multi mode
// regardless of --skill / --exclude, otherwise the product skills reference a
// dingtalk-shared that was never installed.
const multiSharedSkill = "dingtalk-shared"

// ensureMandatorySharedSkill guarantees the shared dependency skill is included
// whenever it exists in the source, even if --skill / --exclude narrowed it out.
func ensureMandatorySharedSkill(selected, all []string) []string {
	hasShared := false
	for _, n := range all {
		if n == multiSharedSkill {
			hasShared = true
			break
		}
	}
	if !hasShared {
		return selected
	}
	for _, n := range selected {
		if n == multiSharedSkill {
			return selected
		}
	}
	return append([]string{multiSharedSkill}, selected...)
}

// normalizeMultiSkillName accepts either the short form (aitable) or the
// full form (dingtalk-aitable) and returns the canonical full form.
// Empty input returns "". Comparison is case-insensitive.
func normalizeMultiSkillName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	if strings.HasPrefix(n, multiSkillPrefix) {
		return n
	}
	return multiSkillPrefix + n
}

// filterMultiSkillNames narrows `all` by include / exclude lists:
//
//   - include + exclude are mutually exclusive (both → error)
//   - names accept short or full form; normalized before matching
//   - unknown names → error, with the available list inlined for discovery
//   - both lists empty → return `all` (install everything)
//   - exclude that drops every name → error (avoid silent no-op install)
//
// The caller is responsible for additive installation: install only the
// returned names, leaving any other already-installed dingtalk-* siblings
// untouched (handled by installMultiSkillToHomes which does not enumerate
// the destination).
func filterMultiSkillNames(all, include, exclude []string) ([]string, error) {
	if len(include) > 0 && len(exclude) > 0 {
		return nil, fmt.Errorf("--skill 与 --exclude 不能同时使用")
	}

	available := make(map[string]struct{}, len(all))
	for _, n := range all {
		available[n] = struct{}{}
	}

	validate := func(raw []string, flagName string) ([]string, error) {
		var normalized []string
		var unknown []string
		seen := make(map[string]bool)
		for _, r := range raw {
			n := normalizeMultiSkillName(r)
			if n == "" {
				continue
			}
			if _, ok := available[n]; !ok {
				unknown = append(unknown, r)
				continue
			}
			if !seen[n] {
				seen[n] = true
				normalized = append(normalized, n)
			}
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("%s 中的以下名称在 multi 源中找不到：%s\n可用列表（共 %d 个）：%s",
				flagName, strings.Join(unknown, ", "), len(all), strings.Join(all, ", "))
		}
		return normalized, nil
	}

	if len(include) > 0 {
		names, err := validate(include, "--skill")
		if err != nil {
			return nil, err
		}
		sort.Strings(names)
		return names, nil
	}
	if len(exclude) > 0 {
		excluded, err := validate(exclude, "--exclude")
		if err != nil {
			return nil, err
		}
		excludedSet := make(map[string]bool, len(excluded))
		for _, n := range excluded {
			excludedSet[n] = true
		}
		var out []string
		for _, n := range all {
			if !excludedSet[n] {
				out = append(out, n)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("--exclude 把全部 %d 个子 skill 都剔除了，没有可装的", len(all))
		}
		return out, nil
	}
	return all, nil
}

// listMultiSkillNames returns sorted names of subdirectories under src that
// contain a SKILL.md file (i.e. valid multi-mode skill bundles).
func listMultiSkillNames(src string) ([]string, error) {
	entries, err := skillSetupReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("无法读取 multi skill 源目录 %s: %w", src, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := skillSetupStat(filepath.Join(src, e.Name(), "SKILL.md")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// resolveSkillSetupMode resolves the mode either from the flag or via an
// interactive prompt. If no TTY is available and no mode was given, returns
// an error rather than silently picking a default.
func resolveSkillSetupMode(mode string, autoYes bool, out io.Writer) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case skillSetupModeMono, skillSetupModeMulti:
		return mode, nil
	case "":
		// fall through to interactive prompt
	default:
		return "", fmt.Errorf("不支持的 --mode 值: %s（可选 mono / multi）", mode)
	}

	if autoYes || !skillSetupInteractive() {
		fmt.Fprintln(out, "未指定 --mode，非交互环境下默认使用 mono")
		return skillSetupModeMono, nil
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("选择 dws skill 安装模式").
				Description("mono = 单 skill 入口（稳定 / 推荐）\nmulti = 按产品拆分的独立 skill").
				Options(
					huh.NewOption("mono — 单 skill（稳定 / 推荐）", skillSetupModeMono),
					huh.NewOption("multi — 多 skill（按产品拆分）", skillSetupModeMulti),
				).
				Value(&choice),
		),
	)
	if err := skillSetupRunForm(form); err != nil {
		return "", fmt.Errorf("交互式选择中止: %w", err)
	}
	return choice, nil
}

// resolveSkillSetupSource finds the local skill source directory for the
// given mode. PR 1 supports only mono; multi is reserved for a later PR
// and currently returns an error before reaching this function.
func resolveSkillSetupSource(explicit, mode string) (string, error) {
	subdir := mode // "mono" or "multi"

	// An explicit override (--source flag or DWS_SKILL_SOURCE) wins, and an
	// override that does not contain a skill root is an error — never a
	// silent fallback to another source the user did not ask for.
	var overrides []string
	if explicit != "" {
		overrides = append(overrides, explicit, filepath.Join(explicit, "skills", subdir))
	}
	if env := strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")); env != "" {
		overrides = append(overrides, env, filepath.Join(env, "skills", subdir))
	}
	if len(overrides) > 0 {
		for _, c := range overrides {
			if isSkillSourceRoot(c, mode) {
				return c, nil
			}
		}
		hint := strings.Join(overrides, "\n  - ")
		return "", fmt.Errorf("未找到 %s 模式的 skill 源目录（--source / DWS_SKILL_SOURCE 显式指定时不回退到内嵌源），已尝试：\n  - %s", mode, hint)
	}

	// No explicit override: legacy fallback only — embedded materialization
	// is handled by resolveSkillSetupSourceOrEmbedded (skill_setup_embed.go),
	// the wrapper that callers use. This branch is reachable only when the
	// wrapper passes through with an empty explicit/env (legacy direct call).
	candidates := skillSourceCandidates("", subdir)
	for _, c := range candidates {
		if isSkillSourceRoot(c, mode) {
			return c, nil
		}
	}

	hint := strings.Join(candidates, "\n  - ")
	return "", fmt.Errorf("未找到 %s 模式的 skill 源目录，已尝试：\n  - %s\n\n请用 --source 显式指定包含 skills/%s 的仓库根目录", mode, hint, mode)
}

// skillSourceCandidates returns the ordered list of paths to probe for a
// skill source root, given an optional explicit override and the mode
// subdir (mono or multi).
func skillSourceCandidates(explicit, subdir string) []string {
	var roots []string
	if explicit != "" {
		// allow either repo root or already-resolved skills/<mode> dir
		roots = append(roots, explicit, filepath.Join(explicit, "skills", subdir))
	}
	if env := strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")); env != "" {
		roots = append(roots, env, filepath.Join(env, "skills", subdir))
	}
	if exe, err := skillSetupExecutable(); err == nil {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(exeDir, "skills", subdir),
			filepath.Join(exeDir, "..", "skills", subdir),
			filepath.Join(exeDir, "..", "share", "skills", "dws"),
		)
	}
	if wd, err := skillSetupGetwd(); err == nil {
		roots = append(roots, filepath.Join(wd, "skills", subdir))
	}
	// User-level cache populated by install.sh / install.ps1 / npm install.js
	// from the dws-skills.zip release asset. Lets `dws skill setup` find a
	// source even when the user has no source checkout on disk.
	if home, err := skillSetupUserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".dws", "skills", subdir))
	}
	return roots
}

func isSkillSourceRoot(path, mode string) bool {
	if path == "" {
		return false
	}
	switch mode {
	case skillSetupModeMono:
		fi, err := skillSetupStat(filepath.Join(path, "SKILL.md"))
		return err == nil && !fi.IsDir()
	case skillSetupModeMulti:
		entries, err := skillSetupReadDir(path)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := skillSetupStat(filepath.Join(path, e.Name(), "SKILL.md")); err == nil {
					return true
				}
			}
		}
		return false
	}
	return false
}

// resolveSkillSetupTargets returns the list of absolute Agent home destinations.
// If target == "all", returns every agent home whose parent directory exists.
// Otherwise returns the single matching home (whether or not it currently exists).
//
// 末段约定：
//   - mono  → <agent-home>/dws   （单 skill，整个 src 拷成一个 dws 目录）
//   - multi → <agent-home>       （安装时把 src 下每个子目录拷成兄弟 skill）
func resolveSkillSetupTargets(target, mode string) ([]string, error) {
	home, err := skillSetupUserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法解析用户 HOME: %w", err)
	}

	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" || target == "all" {
		return detectExistingAgentHomes(home, mode), nil
	}

	rel, ok := agentSkillPaths[target]
	if !ok {
		return nil, fmt.Errorf("不支持的 --target 值: %s（可选 all, %s）", target, supportedTargets())
	}
	return []string{agentHomeForMode(filepath.Join(home, rel), mode)}, nil
}

// agentHomeForMode appends the mode-specific tail segment to an agent home base.
func agentHomeForMode(base, mode string) string {
	if mode == skillSetupModeMulti {
		return base
	}
	return filepath.Join(base, "dws")
}

func detectExistingAgentHomes(home, mode string) []string {
	var out []string
	for i, rel := range skillSetupAgentHomes {
		base := filepath.Join(home, rel)
		parent := filepath.Dir(base)
		if i > 0 {
			if _, err := skillSetupStat(parent); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		out = append(out, agentHomeForMode(base, mode))
	}
	return out
}

// mutualExclusionVictims returns the paths that should be removed before
// installing into dest under the given mode, to prevent leftover files from
// the opposite mode from co-existing.
//
//   - mono dest is <agent-home>/dws  → multi 残留是 <agent-home>/dingtalk-*
//   - multi dest is <agent-home>     → mono 残留是 <agent-home>/dws
func mutualExclusionVictims(dest, mode string) []string {
	switch mode {
	case skillSetupModeMono:
		// dest = <agent-home>/dws → agent-home = parent
		agentHome := filepath.Dir(dest)
		entries, err := skillSetupReadDir(agentHome)
		if err != nil {
			return nil
		}
		var victims []string
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "dingtalk-") {
				victims = append(victims, filepath.Join(agentHome, e.Name()))
			}
		}
		sort.Strings(victims)
		return victims
	case skillSetupModeMulti:
		// dest = <agent-home> → mono 残留是 dest/dws
		monoPath := filepath.Join(dest, "dws")
		if _, err := skillSetupStat(monoPath); err == nil {
			return []string{monoPath}
		}
		return nil
	}
	return nil
}

type skillSetupAppliedOperation struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Path   string `json:"path"`
	Skill  string `json:"skill,omitempty"`
}

type skillSetupApplyReport struct {
	Succeeded []any
	Failed    []output.PartialFailedEntry
	Unknown   []output.PartialUnknownEntry
}

type skillSetupSuccess struct {
	Mode       string   `json:"mode"`
	Source     string   `json:"source"`
	Targets    []string `json:"targets"`
	Skills     []string `json:"skills"`
	Installed  int      `json:"installed"`
	Removed    int      `json:"removed"`
	Operations []any    `json:"operations"`
}

func skillSetupResult(mode, source string, dests, skillNames []string, report skillSetupApplyReport) output.CommandResult {
	if report.Succeeded == nil {
		report.Succeeded = []any{}
	}
	if report.Failed == nil {
		report.Failed = []output.PartialFailedEntry{}
	}
	if report.Unknown == nil {
		report.Unknown = []output.PartialUnknownEntry{}
	}
	if len(report.Failed) == 0 && len(report.Unknown) == 0 {
		installed, removed := 0, 0
		for _, item := range report.Succeeded {
			operation, ok := item.(skillSetupAppliedOperation)
			if !ok {
				continue
			}
			switch operation.Action {
			case "install":
				installed++
			case "remove":
				removed++
			}
		}
		targets := append([]string(nil), dests...)
		skills := append([]string(nil), skillNames...)
		if skills == nil {
			skills = []string{}
		}
		return output.Success(skillSetupSuccess{
			Mode: mode, Source: source, Targets: targets, Skills: skills,
			Installed: installed, Removed: removed, Operations: report.Succeeded,
		})
	}

	if len(report.Succeeded) > 0 {
		partial, err := output.NewPartialData(
			len(report.Succeeded)+len(report.Failed)+len(report.Unknown),
			report.Succeeded,
			report.Failed,
			report.Unknown,
		)
		if err == nil {
			return output.Partial(partial)
		}
		return output.Failure(&output.ErrorInfo{
			Type: "internal", Subtype: "skill_setup_result_invalid",
			Message: err.Error(),
		})
	}

	// All-failed batches cannot use partial_failure. Preserve every per-path
	// failure/unknown fact in details so an Agent can inspect the filesystem
	// before retrying instead of assuming no mutation occurred.
	return output.Failure(&output.ErrorInfo{
		Type:    "internal",
		Subtype: "skill_setup_failed",
		Message: "skill setup did not complete any operation",
		Hint:    "inspect the listed paths before retrying; unknown entries may have been modified",
		Details: map[string]any{
			"mode": mode, "source": source,
			"failed": report.Failed, "unknown": report.Unknown,
		},
	})
}

func applySkillSetup(mode, src string, skillNames, dests []string) skillSetupApplyReport {
	report := skillSetupApplyReport{
		Succeeded: []any{},
		Failed:    []output.PartialFailedEntry{},
		Unknown:   []output.PartialUnknownEntry{},
	}
	targets := append([]string(nil), dests...)
	sort.Strings(targets)
	for _, dest := range targets {
		if !applySkillSetupMutualExclusion(&report, dest, mode) {
			appendSkillSetupBlockedInstalls(&report, dest, mode, skillNames)
			continue
		}
		switch mode {
		case skillSetupModeMono:
			applySkillSetupOne(&report, src, dest, "dws")
		case skillSetupModeMulti:
			if err := skillSetupMkdirAll(dest, 0o755); err != nil {
				for _, name := range skillNames {
					path := filepath.Join(dest, name)
					appendSkillSetupFailure(&report, "install:"+path, "create_parent", path, err)
				}
				continue
			}
			for _, name := range skillNames {
				applySkillSetupOne(&report, filepath.Join(src, name), filepath.Join(dest, name), name)
			}
		default:
			appendSkillSetupFailure(&report, "mode:"+mode, "validate", mode, fmt.Errorf("unknown mode %q", mode))
		}
	}
	return report
}

func applySkillSetupMutualExclusion(report *skillSetupApplyReport, dest, mode string) bool {
	for _, victim := range mutualExclusionVictims(dest, mode) {
		if err := skillSetupRemoveAll(victim); err != nil {
			appendSkillSetupUnknown(report, "remove:"+victim, fmt.Sprintf("互斥目录删除失败，目录可能已被部分修改: %v", err))
			return false
		}
		report.Succeeded = append(report.Succeeded, skillSetupAppliedOperation{
			ID: "remove:" + victim, Action: "remove", Path: victim,
		})
	}
	return true
}

func appendSkillSetupBlockedInstalls(report *skillSetupApplyReport, dest, mode string, skillNames []string) {
	paths := []struct{ path, skill string }{}
	if mode == skillSetupModeMono {
		paths = append(paths, struct{ path, skill string }{dest, "dws"})
	} else {
		for _, name := range skillNames {
			paths = append(paths, struct{ path, skill string }{filepath.Join(dest, name), name})
		}
	}
	for _, item := range paths {
		appendSkillSetupFailure(report, "install:"+item.path, "failed_precondition", item.path,
			fmt.Errorf("互斥模式残留未能安全清理，已阻止安装以避免 mono/multi 混合状态"))
	}
}

func applySkillSetupOne(report *skillSetupApplyReport, src, dest, skill string) {
	existed, err := skillSetupPathExists(dest)
	if err != nil {
		appendSkillSetupUnknown(report, "inspect:"+dest, fmt.Sprintf("无法确认目标目录现状: %v", err))
		appendSkillSetupFailure(report, "install:"+dest, "failed_precondition", dest,
			fmt.Errorf("无法确认目标目录现状，已阻止覆盖"))
		return
	}
	if err := skillSetupRemoveAll(dest); err != nil {
		appendSkillSetupUnknown(report, "remove:"+dest, fmt.Sprintf("目标目录删除失败，目录可能已被部分修改: %v", err))
		appendSkillSetupFailure(report, "install:"+dest, "failed_precondition", dest,
			fmt.Errorf("目标目录未能安全清理，已阻止安装"))
		return
	}
	if existed {
		report.Succeeded = append(report.Succeeded, skillSetupAppliedOperation{
			ID: "remove:" + dest, Action: "remove", Path: dest, Skill: skill,
		})
	}
	if err := skillSetupMkdirAll(filepath.Dir(dest), 0o755); err != nil {
		appendSkillSetupFailure(report, "install:"+dest, "create_parent", dest, err)
		return
	}
	if err := skillSetupCopyDir(src, dest); err != nil {
		appendSkillSetupUnknown(report, "install:"+dest, fmt.Sprintf("复制过程中失败，目标目录终态未知: %v", err))
		return
	}
	report.Succeeded = append(report.Succeeded, skillSetupAppliedOperation{
		ID: "install:" + dest, Action: "install", Path: dest, Skill: skill,
	})
}

func skillSetupPathExists(path string) (bool, error) {
	_, err := skillSetupStat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func appendSkillSetupFailure(report *skillSetupApplyReport, id, subtype, path string, err error) {
	report.Failed = append(report.Failed, output.PartialFailedEntry{
		ID: id,
		Error: &output.ErrorInfo{
			Type: "internal", Subtype: subtype,
			Message: err.Error(), Details: map[string]any{"path": path},
		},
	})
}

func appendSkillSetupUnknown(report *skillSetupApplyReport, id, reason string) {
	report.Unknown = append(report.Unknown, output.PartialUnknownEntry{ID: id, Reason: reason})
}

// installSkillToHomes and installMultiSkillToHomes are retained for the
// installer-facing compatibility helpers. They delegate to the same strict
// operation engine as `skill setup`; there is no second best-effort path that
// can continue after mutual-exclusion cleanup fails.
func installSkillToHomes(src string, dests []string, out, errOut io.Writer) (installed, skipped int, err error) {
	return renderLegacySkillSetupReport(applySkillSetup(skillSetupModeMono, src, nil, dests), out, errOut)
}

// installMultiSkillToHomes installs each subdir of src (dingtalk-*) into
// dest as a sibling skill directory. installed/skipped is counted per
// (agent-home × sub-skill) pair so the user sees granular progress.
func installMultiSkillToHomes(src string, skillNames []string, dests []string, out, errOut io.Writer) (installed, skipped int, err error) {
	return renderLegacySkillSetupReport(applySkillSetup(skillSetupModeMulti, src, skillNames, dests), out, errOut)
}

func renderLegacySkillSetupReport(report skillSetupApplyReport, out, errOut io.Writer) (installed, skipped int, err error) {
	skippedIDs := map[string]bool{}
	for _, raw := range report.Succeeded {
		operation, ok := raw.(skillSetupAppliedOperation)
		if !ok {
			continue
		}
		switch operation.Action {
		case "install":
			installed++
			fmt.Fprintf(out, "  ✓ %s\n", operation.Path)
		case "remove":
			fmt.Fprintf(out, "  × 已清理对面模式残留/旧版本 %s\n", operation.Path)
		}
	}
	for _, failure := range report.Failed {
		if strings.HasPrefix(failure.ID, "install:") {
			skippedIDs[failure.ID] = true
		}
		message := "unknown failure"
		if failure.Error != nil && failure.Error.Message != "" {
			message = failure.Error.Message
		}
		fmt.Fprintf(errOut, "  ✗ %s: %s\n", failure.ID, message)
	}
	for _, unknown := range report.Unknown {
		if strings.HasPrefix(unknown.ID, "install:") {
			skippedIDs[unknown.ID] = true
		}
		fmt.Fprintf(errOut, "  ⚠️  %s: %s\n", unknown.ID, unknown.Reason)
	}
	return installed, len(skippedIDs), nil
}

func copyDir(src, dst string) error {
	return skillSetupWalk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := skillSetupRel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return skillSetupMkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// resolve symlink target and copy the underlying file
			resolved, err := skillSetupReadlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			}
			return copyFileContent(resolved, target, info.Mode())
		}
		return copyFileContent(path, target, info.Mode())
	})
}

func copyFileContent(src, dst string, mode os.FileMode) error {
	if err := skillSetupMkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := skillSetupOpen(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := skillSetupOpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode&os.ModePerm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = skillSetupCopy(out, in)
	return err
}

func isInteractiveTerminal() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout) && isCharDevice(os.Stderr)
}

func isCharDevice(file *os.File) bool {
	if file == nil {
		return false
	}
	fi, err := file.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
