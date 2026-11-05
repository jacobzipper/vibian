# Vibian: the first login on tty1 goes straight into onboarding.
if [ "$(tty 2>/dev/null)" = /dev/tty1 ] && [ "$(id -u)" -ne 0 ] && command -v vibian >/dev/null; then
  vibian onboard --if-needed
fi
