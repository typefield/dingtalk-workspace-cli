#!/usr/bin/env python3
"""兼容入口：复用 report_inbox_today 的统一结果与分页实现。"""

from report_inbox_today import main
from _runtime import run_main


if __name__ == '__main__':
    raise SystemExit(run_main(main))
