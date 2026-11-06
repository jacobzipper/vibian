# Vibian: the first login on tty1 goes straight into onboarding. Only from a
# person's interactive shell: a login screen can run its own session on tty1
# through this file too (COSMIC's does), and it must not wait for onboarding.
case $- in *i*)
  if [ "$(tty 2>/dev/null)" = /dev/tty1 ] && [ "$(id -u)" -ge 1000 ] && command -v vibian >/dev/null; then
    vibian onboard --if-needed
  fi ;;
esac
