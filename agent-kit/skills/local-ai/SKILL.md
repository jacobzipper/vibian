---
name: local-ai
description: "Run AI models on this machine: Ollama for LLMs, ComfyUI for image/video generation, GPU checks (NVIDIA CUDA, AMD ROCm), keeping multi-GB models out of snapshots, and sharing one GPU between them. Use when the user wants a local model, offline AI, a local backend for their agent harness, Stable Diffusion/Flux-style generation, or an AI tool runs out of VRAM."
---

# Local AI on Vibian

## Check the GPU first
```bash
vibian doctor | grep -i gpu
nvidia-smi                                  # NVIDIA: driver loaded, VRAM free
rocminfo | grep -m3 -i gfx; rocm-smi        # AMD (apt install rocminfo rocm-smi)
```
No NVIDIA driver yet: `drivers-hardware` first. Without a GPU, small models (3-8B, quantised) run on the CPU slowly; say so before pulling a 30 GB model.

| VRAM | realistic |
|---|---|
| CPU only / 8 GB | 3-8B LLMs at Q4, SD 1.5 / SDXL-turbo |
| 12-16 GB | 14B LLMs, SDXL, Flux at fp8 |
| 24 GB+ | 30B+ LLMs at Q4, Flux full, video models |

## Keep models out of snapshots (do this first)
Snapper snapshots `/` and `/home` (hourly for `/home`), so a model deleted today stays on disk for a week, and every model under `/usr` or `/var/lib` sits inside root rollbacks. Give models their own btrfs subvolume, like `/var/lib/docker` has:
```bash
vibian snap "models subvolume"
dev=$(findmnt -no SOURCE / | sed 's/\[.*//'); uuid=$(findmnt -no UUID /)
sudo mount -o subvolid=5 "$dev" /mnt && sudo btrfs subvolume create /mnt/@models && sudo umount /mnt
opts=$(awk '$2=="/" {print $4}' /etc/fstab | sed 's|subvol=/@|subvol=/@models|')
echo "UUID=$uuid /var/lib/models btrfs $opts 0 0" | sudo tee -a /etc/fstab
sudo mkdir -p /var/lib/models && sudo systemctl daemon-reload && sudo mount /var/lib/models
sudo install -d -o "$USER" -g "$USER" /var/lib/models/comfyui
vibian log add "models subvolume at /var/lib/models" --files /etc/fstab --gotcha "@models is outside snapper on purpose"
```

## Ollama (LLMs)
Not in the Ubuntu archive. Use the official installer, which adds a systemd service, an `ollama` user, and the CUDA or ROCm runtime it needs:
```bash
curl -fsSL https://ollama.com/install.sh | sh
sudo install -d -o ollama -g ollama /var/lib/models/ollama
sudo mkdir -p /etc/systemd/system/ollama.service.d
printf '[Service]\nEnvironment=OLLAMA_MODELS=/var/lib/models/ollama\n' | sudo tee /etc/systemd/system/ollama.service.d/50-vibian.conf
sudo systemctl daemon-reload && sudo systemctl restart ollama
ollama pull <model>; ollama run <model> "hi"; ollama ps; ollama list
```
- It listens on `127.0.0.1:11434`. `OLLAMA_HOST=0.0.0.0` exposes it to the network without auth: ask first, and firewall it.
- Harnesses with local-model support (opencode, pi, Codex, OpenClaw and others) can point at `http://127.0.0.1:11434/v1` (OpenAI-compatible). Tell the user that small local models follow these skills much worse than hosted frontier models.
- Updating means re-running the install script. Ledger it.

## ComfyUI (images, video)
```bash
sudo apt-get install -y git python3-venv
git clone https://github.com/comfyanonymous/ComfyUI ~/ComfyUI && cd ~/ComfyUI
python3 -m venv .venv && . .venv/bin/activate
pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu128   # NVIDIA; ROCm: …/whl/rocm<ver>
pip install -r requirements.txt
```
- Point its model folders at `/var/lib/models/comfyui` with `extra_model_paths.yaml` (copy the `.example` file), so checkpoints don't land in `/home`.
- Run it as a user service (see `services`): `ExecStart=%h/ComfyUI/.venv/bin/python main.py --listen 127.0.0.1 --port 8188`.
- Pick the torch wheel that matches the driver: `nvidia-smi` shows the newest CUDA it supports.
- Custom nodes run arbitrary Python. Install only ones the user asked for, from their GitHub repos.

## Sharing one GPU
Ollama keeps a model in VRAM for 5 minutes after the last request. Before a big ComfyUI job: `ollama stop <model>` (or `ollama ps` to see what's loaded). To make that the default, `OLLAMA_KEEP_ALIVE=0` in the same drop-in. Running out of memory looks like `CUDA out of memory` / `HIP out of memory`: unload, or use a smaller quant, `--lowvram` in ComfyUI, or fp8 weights.

## Ledger
Log the subvolume, every service and drop-in, and each model's size. Models themselves don't need entries.
