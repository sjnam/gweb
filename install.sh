#!/bin/sh
# install.sh -- install the GWEB tools, macro package, and man page.
#
# Installs:
#   gtangle, gweave        -> BINDIR        (default $PREFIX/bin)
#   gwebmac.tex, kotexgweb.tex
#                          -> TEXMFDIR/tex/plain/gweb  (default your TEXMFHOME)
#   gweb.1                 -> MANDIR        (default $PREFIX/share/man/man1)
#
# Usage:
#   ./install.sh [--prefix=DIR] [--bindir=DIR] [--mandir=DIR] [--texmf=DIR]
#   ./install.sh --uninstall [same dir options]
#
# Environment variables PREFIX, BINDIR, MANDIR, TEXMFDIR, GO override the
# defaults too. A system-wide install usually needs sudo, e.g.
#   sudo ./install.sh --prefix=/usr/local --texmf="$(kpsewhich -var-value TEXMFLOCAL)"
set -eu

cd "$(dirname "$0")"

GO="${GO:-go}"
PREFIX="${PREFIX:-/usr/local}"
BINDIR="${BINDIR:-}"
MANDIR="${MANDIR:-}"
TEXMFDIR="${TEXMFDIR:-}"
uninstall=0

for arg in "$@"; do
	case "$arg" in
	--prefix=*) PREFIX="${arg#*=}" ;;
	--bindir=*) BINDIR="${arg#*=}" ;;
	--mandir=*) MANDIR="${arg#*=}" ;;
	--texmf=*) TEXMFDIR="${arg#*=}" ;;
	--uninstall) uninstall=1 ;;
	-h | --help)
		awk 'NR==1{next} /^#/{sub(/^# ?/,"");print;next} {exit}' "$0"
		exit 0
		;;
	*)
		echo "install.sh: unknown argument '$arg' (try --help)" >&2
		exit 2
		;;
	esac
done

# Derive the remaining defaults after --prefix is known.
: "${BINDIR:=$PREFIX/bin}"
: "${MANDIR:=$PREFIX/share/man/man1}"
if [ -z "$TEXMFDIR" ]; then
	TEXMFDIR="$(kpsewhich -var-value TEXMFHOME 2>/dev/null || true)"
	[ -n "$TEXMFDIR" ] || TEXMFDIR="$HOME/texmf"
fi
GWEBMACDIR="$TEXMFDIR/tex/plain/gweb"

if [ "$uninstall" = 1 ]; then
	echo "Uninstalling GWEB..."
	rm -f "$BINDIR/gtangle" "$BINDIR/gweave"
	rm -f "$MANDIR/gweb.1"
	rm -f "$GWEBMACDIR/gwebmac.tex" "$GWEBMACDIR/kotexgweb.tex"
	rmdir "$GWEBMACDIR" 2>/dev/null || true
	[ -f "$TEXMFDIR/ls-R" ] && command -v mktexlsr >/dev/null 2>&1 && mktexlsr "$TEXMFDIR" >/dev/null 2>&1 || true
	echo "Removed gtangle, gweave, the man page, and the TeX macros."
	exit 0
fi

command -v "$GO" >/dev/null 2>&1 || {
	echo "install.sh: the Go toolchain ('$GO') is required to build the commands" >&2
	exit 1
}

echo "Building and installing the commands into $BINDIR ..."
mkdir -p "$BINDIR"
"$GO" build -o "$BINDIR/gtangle" ./cmd/gtangle
# gweave's Go is not committed (cweb tradition); tangle it with the gtangle just
# built, then compile it.
"$BINDIR/gtangle" cmd/gweave/gweave.w >/dev/null
"$GO" build -o "$BINDIR/gweave" ./cmd/gweave

echo "Installing TeX macros into $GWEBMACDIR ..."
mkdir -p "$GWEBMACDIR"
cp gwebmac.tex "$GWEBMACDIR/gwebmac.tex"
# Korean (luatexko) support: kotexgweb.tex is loaded from a .w file's limbo.
# Harmless to install even if you never write Korean webs.
cp kotexgweb.tex "$GWEBMACDIR/kotexgweb.tex"
[ -f "$TEXMFDIR/ls-R" ] && command -v mktexlsr >/dev/null 2>&1 && mktexlsr "$TEXMFDIR" >/dev/null 2>&1 || true

echo "Installing the man page into $MANDIR ..."
mkdir -p "$MANDIR"
cp gweb.1 "$MANDIR/"

echo
echo "Done. Installed:"
echo "  $BINDIR/gtangle"
echo "  $BINDIR/gweave"
echo "  $GWEBMACDIR/gwebmac.tex, kotexgweb.tex"
echo "  $MANDIR/gweb.1"
echo
case ":$PATH:" in
*":$BINDIR:"*) ;;
*) echo "Note: $BINDIR is not on your PATH; add it to run gtangle/gweave." ;;
esac
if command -v kpsewhich >/dev/null 2>&1; then
	# From / , not here: gwebmac.tex sits in this directory, and kpsewhich looks
	# in the current one first, which would find our own copy and say nothing
	# about whether the installed one is on TeX's search path.
	if found="$(cd / && kpsewhich gwebmac.tex 2>/dev/null)" && [ -n "$found" ]; then
		echo "TeX finds the macros at: $found"
	else
		echo "Note: TeX did not find gwebmac.tex on its own. Either set"
		echo "  export TEXINPUTS=\"$GWEBMACDIR:\""
		echo "or install into a TeX tree that is on your TEXMF search path."
	fi
fi
