#!/usr/bin/env bash

set -e

ROOT="internal"
PUML_DIR="docs/puml"
SVG_DIR="docs/svg"
STRUCT_MD="docs/structure.md"
PLANTUML_URL="http://localhost:8080"
CONTAINER_NAME="plantuml-local"

mkdir -p "$PUML_DIR" "$SVG_DIR"

# detect container runtime
if command -v podman >/dev/null 2>&1; then
    RUNTIME="podman"
elif command -v docker >/dev/null 2>&1; then
    RUNTIME="docker"
else
    echo "[ERROR] neither podman nor docker found"
    exit 1
fi

if ! command -v goplantuml >/dev/null 2>&1; then
    echo "[ERROR] goplantuml not found in PATH"
    exit 1
fi

# start plantuml server if not already running
if ! curl -s -o /dev/null "$PLANTUML_URL"; then
    echo "[INFO] plantuml server not running, starting via $RUNTIME"

    if $RUNTIME ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        $RUNTIME start "$CONTAINER_NAME"
    else
        $RUNTIME run -d --name "$CONTAINER_NAME" -p 8080:8080 plantuml/plantuml-server:jetty
    fi

    echo "[INFO] waiting for plantuml server to become ready"
    for i in $(seq 1 15); do
        if curl -s -o /dev/null "$PLANTUML_URL"; then
            break
        fi
        sleep 1
    done

    if ! curl -s -o /dev/null "$PLANTUML_URL"; then
        echo "[ERROR] plantuml server did not start in time"
        exit 1
    fi
fi

echo "# Package Structure" > "$STRUCT_MD"
echo "" >> "$STRUCT_MD"
echo "Generated from $ROOT on $(date +%Y-%m-%d)" >> "$STRUCT_MD"
echo "" >> "$STRUCT_MD"

for pkg_dir in "$ROOT"/*/; do
    pkg=$(basename "$pkg_dir")

    echo "[INFO] processing $pkg"

    puml_file="$PUML_DIR/$pkg.puml"
    svg_file="$SVG_DIR/$pkg.svg"

    goplantuml \
        -show-aggregations \
        -show-compositions \
        -show-connection-labels \
        "$pkg_dir" > "$puml_file" 2>/dev/null

    if [ ! -s "$puml_file" ]; then
        echo "[FAIL] $pkg produced empty puml, skipping"
        rm -f "$puml_file"
        continue
    fi

    sed -i '1a top to bottom direction' "$puml_file"

    status=$(curl -s -X POST \
        -H "Content-Type: text/plain" \
        --data-binary @"$puml_file" \
        "$PLANTUML_URL/svg" -o "$svg_file" -w "%{http_code}")

    if [ "$status" != "200" ] || [ ! -s "$svg_file" ]; then
        echo "[FAIL] $pkg svg render failed"
        continue
    fi

    echo "[SUCCESS] $pkg -> $svg_file"

    {
        echo "## $pkg"
        echo ""
        echo "\`internal/$pkg\`"
        echo ""
        echo "<img src=\"./svg/$pkg.svg\" width=\"700\">"
        echo ""
    } >> "$STRUCT_MD"
done

echo "[SUCCESS] wrote $STRUCT_MD"

# cleanup: only kill the container if this script started it
if [ "$STARTED_CONTAINER" -eq 1 ]; then
    echo "[INFO] stopping $CONTAINER_NAME"
    $RUNTIME stop "$CONTAINER_NAME" >/dev/null
    echo "[SUCCESS] container stopped"
fi
