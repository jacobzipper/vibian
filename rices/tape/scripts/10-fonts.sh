#!/bin/bash
# IBM Plex Mono (SIL Open Font Licence) into ~/.local/share/fonts/tape:
# regular, medium and bold. Pinned and checked.
set -euo pipefail
dir=${XDG_DATA_HOME:-$HOME/.local/share}/fonts/tape
mkdir -p "$dir"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
base=https://raw.githubusercontent.com/google/fonts/01d5d9f05246fb7090a8403b81257535cb1a088b/ofl/ibmplexmono
while read -r sum name; do
  [ -e "$dir/$name" ] && continue
  curl -fsSL --retry 3 -o "$tmp/$name" "$base/$name"
  echo "$sum  $tmp/$name" | sha256sum -c --quiet
  mv "$tmp/$name" "$dir/"
done <<'LIST'
6a3412f058c7d8dfd9170c41e85ade48e5156ecb89356110ca57a0a27734af46 IBMPlexMono-Regular.ttf
a9b4c49bb299e05b5f6c481e7fb5e78943d2793249a0c8874ab574a2d1ea6755 IBMPlexMono-Medium.ttf
ac27abd6450a64dd94467580a02fe6235156d5b92f2926ebbc8e7489df64e0be IBMPlexMono-Bold.ttf
7e6b2818edbd8f6a01ae80641cc8f16a51080d08fb4e532be3a0b6f74adb07da OFL.txt
LIST
fc-cache -f "$dir"
families=$(fc-list : family)
grep -qF "IBM Plex Mono" <<<"$families" || { echo "IBM Plex Mono didn't install" >&2; exit 1; }
echo "fonts: IBM Plex Mono in $dir"
