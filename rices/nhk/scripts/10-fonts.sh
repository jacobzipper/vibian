#!/bin/bash
# The rice's fonts, all under the SIL Open Font Licence, into
# ~/.local/share/fonts/nhk: Dela Gothic One (titles), DotGothic16 (the
# clock) and Maple Mono NF (text and the terminal). Pinned and checked.
set -euo pipefail
dir=${XDG_DATA_HOME:-$HOME/.local/share}/fonts/nhk
mkdir -p "$dir"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
get() {  # get URL SHA256 NAME: a .ttf, or a .zip unpacked into NAME/
  [ -e "$dir/$3" ] && return 0
  curl -fsSL --retry 3 -o "$tmp/dl" "$1"
  echo "$2  $tmp/dl" | sha256sum -c --quiet
  case $1 in
    *.zip) unzip -qo "$tmp/dl" '*.ttf' LICENSE.txt -d "$tmp/$3" && mv "$tmp/$3" "$dir/" ;;
    *) mv "$tmp/dl" "$dir/$3" ;;
  esac
}
gf=https://raw.githubusercontent.com/google/fonts
get $gf/690b65d64dabaebf9d18e2d7e8c8e7e02b4cccaf/ofl/delagothicone/DelaGothicOne-Regular.ttf \
  4ff87a0965f1b0505e5a2c58424bc6ad3cff27e56a82f21c2fc9d6b0e3857ee2 DelaGothicOne-Regular.ttf
get $gf/9a6cc6b8ce992aff77b69c857c46af0b42cdff76/ofl/dotgothic16/DotGothic16-Regular.ttf \
  3ad9af88726d42b40f7f365f0dcac785af73cf20ea6f1d5b44e57cc21150b8f1 DotGothic16-Regular.ttf
get https://github.com/subframe7536/maple-font/releases/download/v7.9/MapleMono-NF-unhinted.zip \
  8248d6260660be1066b5ebe90d25131582df8383f38db1c284dd1f2dc9d2b46e MapleMono-NF-7.9
fc-cache -f "$dir"
# read the list once: grep -q on a pipe would SIGPIPE fc-list under pipefail
families=$(fc-list : family)
for f in "Dela Gothic One" DotGothic16 "Maple Mono NF"; do
  grep -qF -- "$f" <<<"$families" || { echo "font $f didn't install" >&2; exit 1; }
done
echo "fonts: Dela Gothic One, DotGothic16, Maple Mono NF in $dir"
