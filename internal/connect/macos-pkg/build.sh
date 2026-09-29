#!/bin/sh
# Build — and, when signing credentials are present, sign + notarize — the
# DeepRouter macOS installer (PRD §11.7). macOS only: pkgbuild/productbuild
# ship with the Xcode command line tools, which is why this runs on a GitHub
# macOS runner and not in the gateway's own build.
#
#   sh build.sh [outdir]        # default outdir: dist
#
# Signing is opt-in via environment (see macos-pkg.yml for the secret names):
#   DR_SIGN_IDENTITY   "Developer ID Installer: <name> (<team id>)"
#   DR_NOTARY_KEY_FILE path to the App Store Connect API .p8
#   DR_NOTARY_KEY_ID / DR_NOTARY_ISSUER
# Without DR_SIGN_IDENTITY the output is deeprouter-setup-unsigned.pkg — a
# test artifact Gatekeeper will block on double-click (right-click → Open, or
# `installer -pkg … -target CurrentUserHomeDirectory` to try it anyway).
set -eu
cd "$(dirname "$0")"

OUT="${1:-dist}"
VERSION="${DR_PKG_VERSION:-1.0.0}"
IDENTIFIER="co.deeprouter.setup"

mkdir -p "$OUT"

# Scripts-only component: nothing lands on disk, the postinstall does the work.
# If user-domain installs of a payload-free package misbehave on some macOS
# release (the known open risk — real-machine verification is the boss's step),
# the fallback is a minimal payload into ~/.deeprouter/ — see the P8 card.
pkgbuild --nopayload \
  --scripts scripts \
  --identifier "$IDENTIFIER" \
  --version "$VERSION" \
  "$OUT/component.pkg"

# enable_currentUserHome and nothing else: a user-domain install is what keeps
# the admin-password prompt away, and a password prompt would break the whole
# "no typing" promise (PRD §11 D5).
cat > "$OUT/distribution.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
  <title>DeepRouter Setup</title>
  <domains enable_currentUserHome="true" enable_localSystem="false" enable_anywhere="false"/>
  <options customize="never" require-scripts="true" hostArchitectures="arm64,x86_64"/>
  <conclusion file="conclusion.html"/>
  <choices-outline>
    <line choice="default"/>
  </choices-outline>
  <choice id="default" title="DeepRouter Setup">
    <pkg-ref id="$IDENTIFIER"/>
  </choice>
  <pkg-ref id="$IDENTIFIER" version="$VERSION">component.pkg</pkg-ref>
</installer-gui-script>
XML

productbuild \
  --distribution "$OUT/distribution.xml" \
  --resources resources \
  --package-path "$OUT" \
  "$OUT/deeprouter-setup-unsigned.pkg"

if [ -z "${DR_SIGN_IDENTITY:-}" ]; then
  echo "build.sh: no DR_SIGN_IDENTITY - leaving $OUT/deeprouter-setup-unsigned.pkg unsigned"
  exit 0
fi

productsign --sign "$DR_SIGN_IDENTITY" \
  "$OUT/deeprouter-setup-unsigned.pkg" "$OUT/deeprouter-setup.pkg"

if [ -n "${DR_NOTARY_KEY_FILE:-}" ]; then
  xcrun notarytool submit "$OUT/deeprouter-setup.pkg" \
    --key "$DR_NOTARY_KEY_FILE" \
    --key-id "$DR_NOTARY_KEY_ID" \
    --issuer "$DR_NOTARY_ISSUER" \
    --wait
  # The staple is what lets an offline Mac verify the notarization.
  xcrun stapler staple "$OUT/deeprouter-setup.pkg"
else
  echo "build.sh: signed but NOT notarized - Gatekeeper will still warn on first run"
fi

echo "build.sh: done -> $OUT/deeprouter-setup.pkg"
