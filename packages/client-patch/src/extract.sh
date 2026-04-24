#!/usr/bin/env bash

set -e

BASE_DIR=$(pwd)
WORK_DIR=$BASE_DIR/temp
PSEUDO_DEVICES_FILE=$WORK_DIR/pseudo-devices.txt

FIRMWARE=$(basename $(ls $BASE_DIR/assets/*.bin 2>/dev/null | head -n 1) .bin)

if [ ! -f "$BASE_DIR/assets/$FIRMWARE.bin" ]; then
    echo "❌ 固件文件不存在，请先下载固件到：$BASE_DIR/assets/"
    exit 1
fi

rm -rf "$WORK_DIR" && mkdir -pv "$WORK_DIR" && cd $WORK_DIR

python3 $BASE_DIR/src/extract.py -e "$BASE_DIR/assets/$FIRMWARE.bin" -d "$WORK_DIR/$FIRMWARE"

ln -sf $WORK_DIR/$FIRMWARE/root.squashfs $WORK_DIR/root.squashfs 

unsquashfs -lln $WORK_DIR/root.squashfs | awk '
function perm_to_octal(perm,   i, digit, triad, value, out) {
    perm = substr(perm, 2);
    out = "";
    for (i = 1; i <= 9; i += 3) {
        triad = substr(perm, i, 3);
        value = 0;
        if (substr(triad, 1, 1) == "r") value += 4;
        if (substr(triad, 2, 1) == "w") value += 2;
        if (substr(triad, 3, 1) ~ /[xsStT]/) value += 1;
        out = out value;
    }
    return out;
}
/^[cb]/ {
    split($2, owner, "/");
    major = $3;
    gsub(/,/, "", major);
    path = $7;
    sub(/^squashfs-root\//, "", path);
    printf "%s %s %s %s %s %s %s\n", path, substr($1, 1, 1), perm_to_octal($1), owner[1], owner[2], major, $4;
}' > "$PSEUDO_DEVICES_FILE"

unsquashfs -no-exit-code $WORK_DIR/root.squashfs
