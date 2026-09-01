#!/usr/bin/env bash
# Enforcer — Platform detection helpers
# Author: Deepankar Das
#
# Source this file from any deploy/setup script:
#     . "$(dirname "$0")/lib/platform.sh"
#     parse_env_flag "$@"; set -- "${REMAINING_ARGS[@]}"
#     case "$(detect_platform)" in macos) ... ;; linux) ... ;; esac
#
# Provides:
#   parse_env_flag "$@"    Strips a --env <local_macos|local_ubuntu> flag from
#                          the args and sets AA_PLATFORM_OVERRIDE. The remaining
#                          args land in the REMAINING_ARGS array.
#   detect_platform        Echoes "macos" | "linux" | "unsupported".
#                          Honors AA_PLATFORM_OVERRIDE; otherwise uses uname -s.
#   root_group             "wheel" on macOS, "root" on Linux.
#   managed_hooks_path     OS-specific Claude Code managed-settings.json path.

AA_PLATFORM_OVERRIDE="${AA_PLATFORM_OVERRIDE:-${AA_PLATFORM:-}}"
REMAINING_ARGS=()

parse_env_flag() {
    local args=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --env)
                shift
                _set_platform_override "$1" || return 2
                shift
                ;;
            --env=*)
                _set_platform_override "${1#--env=}" || return 2
                shift
                ;;
            *)
                args+=("$1")
                shift
                ;;
        esac
    done
    REMAINING_ARGS=("${args[@]}")
    export AA_PLATFORM_OVERRIDE
}

_set_platform_override() {
    case "$1" in
        local_macos|macos|darwin)   AA_PLATFORM_OVERRIDE="macos" ;;
        local_ubuntu|ubuntu|linux)  AA_PLATFORM_OVERRIDE="linux" ;;
        "")
            echo "Error: --env requires a value (local_macos | local_ubuntu)" >&2
            return 1
            ;;
        *)
            echo "Error: unknown --env value '$1' (expected: local_macos | local_ubuntu)" >&2
            return 1
            ;;
    esac
}

detect_platform() {
    if [[ -n "$AA_PLATFORM_OVERRIDE" ]]; then
        echo "$AA_PLATFORM_OVERRIDE"
        return 0
    fi
    case "$(uname -s)" in
        Darwin) echo "macos" ;;
        Linux)  echo "linux" ;;
        *)      echo "unsupported" ;;
    esac
}

root_group() {
    case "$(detect_platform)" in
        macos) echo "wheel" ;;
        *)     echo "root" ;;
    esac
}

managed_hooks_path() {
    case "$(detect_platform)" in
        macos) echo "/Library/Application Support/ClaudeCode/managed-settings.json" ;;
        *)     echo "/etc/claude-code/managed-settings.json" ;;
    esac
}
