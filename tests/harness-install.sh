#!/bin/bash
# harness-install.sh VIBIAN_BASE_DEB
#
# Installs each agent harness with its official installer the way
# `vibian agent set` does, in a fresh Ubuntu 26.04 container, then checks
# that the binary runs and that Vibian's brief, MCP servers and turn hooks
# are where that harness version reads them. Harnesses change their
# installers, paths and flags often; this runs weekly in CI to catch it.
# Run as root in a throwaway container:
#   docker run --rm -v "$PWD:/src:ro" ubuntu:26.04 bash /src/tests/harness-install.sh /src/out/vibian-base_0.7.2_amd64.deb
# Prints harness.<id>=ok or harness.<id>=FAIL (why) per harness, and exits 1
# if any failed. HARNESSES limits the run (space-separated ids).
set -uo pipefail
deb=${1:?usage: harness-install.sh VIBIAN_BASE_DEB}
[ "$(id -u)" = 0 ] || { echo "run as root in a throwaway container" >&2; exit 1; }
export DEBIAN_FRONTEND=noninteractive
apt-get update -q >/dev/null && apt-get install -y -q curl ca-certificates sudo git unzip xz-utils >/dev/null || exit 1
# the package's files without its maintainer scripts: those set up
# snapshots and networking, which a container has no use for
dpkg-deb -x "$deb" / || exit 1
useradd -m -s /bin/bash -G sudo agent && echo 'agent ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/agent

cat > /tmp/check.sh <<'EOF'
#!/bin/bash
# check.sh ID, as the agent user: install, configure, inspect
set -uo pipefail
id=$1 h=$HOME
export PATH=$h/.local/bin:$h/.opencode/bin:$h/.npm-global/bin:$PATH
fail() { echo "harness.$id=FAIL ($*)"; exit 1; }
has() { [ -f "$1" ] && grep -q -- "$2" "$1" || fail "$3"; }
# shows PATTERN CMD...: CMD's output mentions PATTERN. Not a pipe into
# grep -q: under pipefail that SIGPIPEs CMD when the match comes early.
# Tries 5 times: opencode's background server loads its config after the
# first `mcp list` calls in a fresh home, which answer "none configured".
shows() {
  local out i
  for i in 1 2 3 4 5; do
    out=$(timeout 60 "${@:2}" </dev/null 2>&1)
    grep -qi -- "$1" <<<"$out" && return 0
    sleep 3
  done
  sed "s/^/  $id: /" <<<"$out" | tail -n 10
  return 1
}
timeout 15m vibian agent set "$id" > "/tmp/set-$id.log" 2>&1 || { tail -n 20 "/tmp/set-$id.log" | sed "s/^/  $id: /"; fail "vibian agent set"; }
list=$(vibian agent list 2>&1)
grep -Eq "^\* $id +.* installed$" <<<"$list" || { sed "s/^/  $id: /" <<<"$list"; fail "not listed as installed"; }
bin=$id; [ "$id" = cursor ] && bin=agent
command -v "$bin" >/dev/null || fail "$bin isn't on PATH"
timeout 60 "$bin" --version </dev/null >/dev/null 2>&1 || fail "$bin --version"
mark=' hook turn-start --agent '$id
case $id in
  claude)
    has $h/.claude/CLAUDE.md /usr/share/vibian/agent/AGENTS.md "brief"
    has $h/.claude.json vibian-system "MCP servers"
    has $h/.claude/settings.json "$mark" "hooks"
    shows vibian-system claude mcp list || fail "claude mcp list doesn't show vibian-system" ;;
  codex)
    has $h/.codex/AGENTS.md vibian:begin "brief"
    has $h/.codex/config.toml vibian-system "MCP servers"
    has /etc/codex/requirements.toml " hook turn-start" "managed hooks"
    shows vibian-system codex mcp list || fail "codex mcp list doesn't show vibian-system" ;;
  gemini)
    has $h/.gemini/GEMINI.md vibian:begin "brief"
    has $h/.gemini/settings.json vibian-system "MCP servers"
    has $h/.gemini/settings.json "$mark" "hooks"
    shows vibian-system gemini mcp list || fail "gemini mcp list doesn't show vibian-system" ;;
  opencode)
    has $h/.config/opencode/AGENTS.md vibian:begin "brief"
    has /etc/opencode/opencode.json vibian-system "MCP servers"
    has $h/.config/opencode/plugins/vibian.ts '"turn-start"' "plugin"
    shows vibian-system opencode mcp list || fail "opencode mcp list doesn't show vibian-system" ;;
  openclaw)
    has $h/.openclaw/workspace/AGENTS.md vibian:begin "brief"
    has $h/.openclaw/openclaw.json vibian-system "MCP servers"
    has $h/.openclaw/openclaw.json /usr/lib/vibian/openclaw "plugin"
    shows vibian openclaw plugins list || fail "openclaw plugins list doesn't show vibian" ;;
  pi)
    has $h/.pi/agent/AGENTS.md vibian:begin "brief"
    has $h/.pi/agent/mcp.json vibian-system "MCP servers"
    has $h/.pi/agent/extensions/vibian.ts '"turn-start"' "extension" ;;
  cursor)
    has $h/AGENTS.md vibian:begin "brief"
    has $h/.cursor/mcp.json vibian-system "MCP servers"
    has $h/.cursor/hooks.json "$mark" "hooks" ;;
esac
echo "harness.$id=ok"
EOF
chmod 755 /tmp/check.sh

bad=0
for id in ${HARNESSES:-claude codex gemini opencode openclaw pi cursor}; do
  echo "== $id $(date -u +%T)"
  su - agent -c "/tmp/check.sh $id" || bad=1
done
exit $bad
