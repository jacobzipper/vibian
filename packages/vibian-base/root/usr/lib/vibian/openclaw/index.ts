// Vibian turn snapshots for OpenClaw: `vibian hook turn-start` before each
// agent run and `turn-end` after it. A hook that throws or times out would
// block the run, so failures are swallowed; vibian logs its own errors.
const hook = (api: any, ev: string) =>
  Promise.resolve()
    .then(() =>
      api.runtime.system.runCommandWithTimeout(["/usr/bin/vibian", "hook", ev, "--agent", "openclaw"], {
        timeoutMs: 12000,
      }),
    )
    .catch(() => {})

export default {
  id: "vibian",
  name: "Vibian",
  description: "Snapshots the system around each agent turn.",
  register(api: any) {
    api.on("before_agent_run", async () => {
      await hook(api, "turn-start")
    })
    api.on("agent_end", async () => {
      await hook(api, "turn-end")
    })
  },
}
