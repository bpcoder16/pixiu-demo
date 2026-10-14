#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_DIR}"' EXIT
WORK_DIR="${TEST_DIR}/output with spaces"
MOCK_NAME="custom-service"
mkdir -p "${WORK_DIR}/bin" "${TEST_DIR}/elsewhere"
cp "${ROOT_DIR}/package/bin/control" "${WORK_DIR}/bin/control"
printf '%s\n' "${MOCK_NAME}" > "${WORK_DIR}/bin/.binary-name"
printf '#!/usr/bin/env bash\nexit 0\n' > "${WORK_DIR}/bin/${MOCK_NAME}"

# 模拟 supervise、进程表和监听端口；测试不启动服务或向真实进程发信号。
cat > "${WORK_DIR}/bin/supervise" <<'SUPERVISE'
#!/usr/bin/env bash
printf '%s\n' "${PWD}" > "${WORK_DIR}/supervise.cwd"
printf '%s\n' "$@" > "${WORK_DIR}/supervise.args"
printf 'start\n' >> "${WORK_DIR}/supervise.calls"
touch "${WORK_DIR}/supervise.running" "${WORK_DIR}/service.running" "${WORK_DIR}/port.open"
SUPERVISE
chmod +x "${WORK_DIR}/bin/control" "${WORK_DIR}/bin/supervise" "${WORK_DIR}/bin/${MOCK_NAME}"

ps() {
    # 同名前缀的其他服务及守护进程不应被识别为本服务。
    printf '900099 %s/bin/supervise -f %s/bin/%s-helper -p %s/status/%s-helper -t 60\n' "${WORK_DIR}" "${WORK_DIR}" "${MOCK_NAME}" "${WORK_DIR}" "${MOCK_NAME}"
    printf '900098 %s/bin/%s-helper\n' "${WORK_DIR}" "${MOCK_NAME}"
    if [[ -f "${WORK_DIR}/supervise.running" ]]; then
        printf '900001 %s/bin/supervise -f %s/bin/%s -p %s/status/%s -t 60\n' "${WORK_DIR}" "${WORK_DIR}" "${MOCK_NAME}" "${WORK_DIR}" "${MOCK_NAME}"
    fi
    if [[ -f "${WORK_DIR}/service.running" ]]; then
        printf '900002 %s/bin/%s\n' "${WORK_DIR}" "${MOCK_NAME}"
    fi
    return 0
}

ss() {
    printf 'State Recv-Q Send-Q Local-Address:Port\n'
    if [[ -f "${WORK_DIR}/port.open" ]]; then
        printf 'LISTEN 0 128 0.0.0.0:%s\n' "${EXPECTED_PORT}"
    fi
    return 0
}

kill() {
    local signal=$1
    local pid=$2
    local marker
    case "${pid}" in
        900001) marker="${WORK_DIR}/supervise.running" ;;
        900002) marker="${WORK_DIR}/service.running" ;;
        *) return 1 ;;
    esac
    if [[ "${signal}" == "-0" ]]; then
        [[ -f "${marker}" ]]
        return $?
    fi
    case "${signal}" in
        -9|-KILL)
            printf 'KILL %s\n' "${pid}" >> "${WORK_DIR}/signals"
            rm -f "${marker}"
            ;;
        -15|-TERM)
            printf 'TERM %s\n' "${pid}" >> "${WORK_DIR}/signals"
            touch "${WORK_DIR}/terminating"
            rm -f "${WORK_DIR}/port.open"
            ;;
        *) return 1 ;;
    esac
}

sleep() {
    if [[ -f "${WORK_DIR}/terminating" ]]; then
        MOCK_TICKS=$((${MOCK_TICKS:-0} + 1))
        if [[ "${MOCK_HANG:-0}" == "0" && "${MOCK_TICKS}" -ge 2 ]]; then
            rm -f "${WORK_DIR}/service.running"
        fi
    else
        # 等待后台的模拟 supervise 完成启动，避免测试依赖调度顺序。
        local attempt=0
        while [[ ! -f "${WORK_DIR}/service.running" && "${attempt}" -lt 100 ]]; do
            /bin/sleep 0.01
            attempt=$((attempt + 1))
        done
    fi
}

export -f ps ss kill sleep
export WORK_DIR MOCK_NAME
unset PROCESS_NAME PORT_MAIN
export EXPECTED_PORT=8888
cd "${TEST_DIR}/elsewhere"

run_control() {
    "${WORK_DIR}/bin/control" "$@" > "${WORK_DIR}/result.log" 2>&1
}

fail() {
    cat "${WORK_DIR}/result.log" >&2
    echo "FAIL: $*" >&2
    exit 1
}

# 自定义程序名及部署目录，并防止已存在但尚未监听的进程被重复启动。
run_control start || fail "start failed"
[[ "$(cat "${WORK_DIR}/supervise.cwd")" == "${WORK_DIR}" ]] || fail "service should start in the output directory"
grep -F -- "${WORK_DIR}/bin/${MOCK_NAME}" "${WORK_DIR}/supervise.args" >/dev/null || fail "start should use the packaged binary name"
rm -f "${WORK_DIR}/port.open"
if run_control start; then
    fail "an existing process without a listening port should not be started again"
fi
[[ "$(wc -l < "${WORK_DIR}/supervise.calls")" -eq 1 ]] || fail "startup should not duplicate a warming process"
touch "${WORK_DIR}/port.open"

# HTTP 端口关闭后仍等待进程清理，先停止守护进程，再只发送一次 SIGTERM。
run_control stop || fail "stop failed"
[[ ! -e "${WORK_DIR}/service.running" ]] || fail "stop should wait for process cleanup after the port closes"
[[ "$(cat "${WORK_DIR}/signals")" == $'KILL 900001\nTERM 900002' ]] || fail "graceful shutdown should send only the expected signals"

# 进程未在优雅退出期限内结束时，再发送一次 SIGKILL。
rm -f "${WORK_DIR}/signals" "${WORK_DIR}/terminating"
touch "${WORK_DIR}/service.running" "${WORK_DIR}/port.open"
MOCK_HANG=1 run_control stop || fail "stop should force exit when the graceful timeout expires"
[[ ! -e "${WORK_DIR}/service.running" ]] || fail "forced shutdown should stop the service"
[[ "$(cat "${WORK_DIR}/signals")" == $'TERM 900002\nKILL 900002' ]] || fail "timeout should send SIGTERM followed by SIGKILL once"

echo 'control_test: ok'
