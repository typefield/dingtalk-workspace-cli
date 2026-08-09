"""将 `dws minutes list mine|shared|all` 的 JSON 规范为 (taskUuid, title) 列表。"""

from __future__ import annotations

import json
from typing import Any, List, Optional, Tuple


def _unwrap_rows(payload: Any) -> Tuple[bool, List[Any]]:
    if isinstance(payload, list):
        return True, payload
    if not isinstance(payload, dict):
        return False, []
    for key in ('itemList', 'items', 'list', 'records', 'minutes'):
        value = payload.get(key)
        if isinstance(value, list):
            return True, value
    for key in ('result', 'data', 'list'):
        value = payload.get(key)
        if isinstance(value, list):
            return True, value
        if isinstance(value, dict):
            for inner_key in (
                'itemList', 'items', 'list', 'records', 'minutes',
            ):
                inner = value.get(inner_key)
                if isinstance(inner, list):
                    return True, inner
    return False, []


def project_uuid_title_pairs(
    payload: Any,
) -> Tuple[List[Tuple[str, str]], Optional[dict[str, Any]]]:
    """Project a known list without dropping malformed or unstable rows."""
    known, rows = _unwrap_rows(payload)
    if not known:
        return [], {
            'type': 'api',
            'subtype': 'projection_unknown',
            'message': '听记列表响应缺少可识别的列表容器。',
        }
    out: List[Tuple[str, str]] = []
    for index, item in enumerate(rows):
        if isinstance(item, dict):
            uuid = (
                item.get('taskUuid') or item.get('taskUUID')
                or item.get('uuid') or item.get('task_uuid')
            )
            title = item.get('title') or item.get('name') or '无标题'
            if not isinstance(uuid, str) or not uuid.strip():
                return [], {
                    'type': 'api',
                    'subtype': 'projection_unknown',
                    'message': f'听记列表第 {index + 1} 项缺少稳定 taskUuid。',
                }
            if not isinstance(title, (str, int, float, bool)):
                return [], {
                    'type': 'api',
                    'subtype': 'projection_unknown',
                    'message': f'听记列表第 {index + 1} 项标题类型不可识别。',
                }
            out.append((uuid.strip(), str(title)))
        elif isinstance(item, str):
            text = item.strip()
            if not text:
                return [], {
                    'type': 'api',
                    'subtype': 'projection_unknown',
                    'message': f'听记列表第 {index + 1} 项为空字符串。',
                }
            if text.startswith('{'):
                try:
                    parsed = json.loads(text)
                except json.JSONDecodeError:
                    return [], {
                        'type': 'api',
                        'subtype': 'projection_unknown',
                        'message': f'听记列表第 {index + 1} 项不是有效 JSON 对象。',
                    }
                if not isinstance(parsed, dict):
                    return [], {
                        'type': 'api',
                        'subtype': 'projection_unknown',
                        'message': f'听记列表第 {index + 1} 项不是对象。',
                    }
                uuid = (
                    parsed.get('taskUuid')
                    or parsed.get('taskUUID')
                    or parsed.get('uuid')
                    or parsed.get('task_uuid')
                )
                title = parsed.get('title') or parsed.get('name') or '无标题'
                if not isinstance(uuid, str) or not uuid.strip():
                    return [], {
                        'type': 'api',
                        'subtype': 'projection_unknown',
                        'message': f'听记列表第 {index + 1} 项缺少稳定 taskUuid。',
                    }
                if not isinstance(title, (str, int, float, bool)):
                    return [], {
                        'type': 'api',
                        'subtype': 'projection_unknown',
                        'message': f'听记列表第 {index + 1} 项标题类型不可识别。',
                    }
                out.append((uuid.strip(), str(title)))
            else:
                out.append((text, text))
        else:
            return [], {
                'type': 'api',
                'subtype': 'projection_unknown',
                'message': f'听记列表第 {index + 1} 项类型不可识别。',
            }
    return out, None


def unwrap_child_data(payload: Any) -> Any:
    """Unwrap a coherent unified child envelope, preserving legacy payloads."""
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def project_summary_text(
    payload: Any,
) -> Tuple[str, Optional[dict[str, Any]]]:
    """Return a known summary string; arbitrary objects are not summaries."""
    value = unwrap_child_data(payload)
    if isinstance(value, str):
        return value, None
    if not isinstance(value, dict):
        return '', {
            'type': 'api', 'subtype': 'projection_unknown',
            'message': '听记摘要响应不是可识别的字符串或对象。',
        }
    inner = value.get('result', value)
    if isinstance(inner, str):
        return inner, None
    if not isinstance(inner, dict):
        return '', {
            'type': 'api', 'subtype': 'projection_unknown',
            'message': '听记摘要 result 不是可识别的字符串或对象。',
        }
    for key in ('fullSummary', 'summary', 'content'):
        if key in inner:
            if isinstance(inner[key], str):
                return inner[key], None
            return '', {
                'type': 'api', 'subtype': 'projection_unknown',
                'message': f'听记摘要字段 {key} 不是字符串。',
            }
    return '', {
        'type': 'api', 'subtype': 'projection_unknown',
        'message': '听记摘要响应缺少 fullSummary/summary/content。',
    }


def project_todo_items(
    payload: Any,
) -> Tuple[List[dict[str, str]], Optional[dict[str, Any]]]:
    """Project known Todo/action arrays without turning drift into empty data."""
    value = unwrap_child_data(payload)
    inner = value.get('result', value) if isinstance(value, dict) else value
    if isinstance(inner, list):
        raw_items = inner
        mode = 'items'
    elif isinstance(inner, dict) and isinstance(inner.get('dingtalkTodoList'), list):
        raw_items = inner['dingtalkTodoList']
        mode = 'items'
        if not raw_items and isinstance(inner.get('actions'), list):
            raw_items = inner['actions']
            mode = 'actions'
    elif isinstance(inner, dict) and isinstance(inner.get('actions'), list):
        raw_items = inner['actions']
        mode = 'actions'
    else:
        return [], {
            'type': 'api', 'subtype': 'projection_unknown',
            'message': '听记待办响应缺少 dingtalkTodoList/actions 列表。',
        }

    projected: List[dict[str, str]] = []
    for index, item in enumerate(raw_items):
        content: Any = None
        if isinstance(item, dict):
            content = (
                item.get('title') or item.get('content')
                or item.get('text') or item.get('value')
            )
        elif isinstance(item, str):
            text = item.strip()
            if mode == 'actions' and text.startswith('{'):
                try:
                    decoded = json.loads(text)
                except json.JSONDecodeError:
                    decoded = None
                if isinstance(decoded, dict):
                    content = (
                        decoded.get('value') or decoded.get('content')
                        or decoded.get('title')
                    )
                else:
                    content = text
            else:
                content = text
        if not isinstance(content, str) or not content.strip():
            return [], {
                'type': 'api', 'subtype': 'projection_unknown',
                'message': f'听记待办第 {index + 1} 项缺少可识别内容。',
            }
        projected.append({'content': content.strip()})
    return projected, None
