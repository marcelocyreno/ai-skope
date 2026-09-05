#!/usr/bin/env python3
"""Draw the AI Skope action icon at every size Chrome asks for.

The mark is the reticle the whole product is built on: the app mark, and the
cursor the element picker uses. A single geometry scaled down does not survive
16px — the ring turns to mush and the ticks disappear — so each size gets its
own proportions: heavier stroke and ticks joined to the ring when small,
returning to the design's lighter, detached form as there is room for it.

    python3 tools/make-icons.py        # writes icons/icon{16,32,48,128}.png
"""
from PIL import Image, ImageDraw

AMBER = (217, 122, 11, 255)  # --accent, light theme
SS = 8  # supersample factor

# size: (ring radius, stroke, dot radius, tick length, tick width, gap)
# Fractions of the icon size. Small sizes get a heavier stroke and ticks that
# touch the ring, so the shape still reads when it is 16 pixels wide.
GEOMETRY = {
    # At 16px the centre dot fills the ring's hole and the mark reads as a
    # blob, so it is dropped: the four ticks are what make this a reticle
    # rather than a circle, and a hollow centre still reads as something you
    # look through.
    16:  dict(ring=0.28, stroke=0.140, dot=0.000, tick=0.170, tickw=0.140, gap=0.000),
    32:  dict(ring=0.30, stroke=0.105, dot=0.090, tick=0.150, tickw=0.105, gap=0.010),
    48:  dict(ring=0.29, stroke=0.088, dot=0.075, tick=0.150, tickw=0.088, gap=0.020),
    128: dict(ring=0.29, stroke=0.073, dot=0.062, tick=0.146, tickw=0.073, gap=0.030),
}


def draw(size: int) -> Image.Image:
    g = GEOMETRY[size]
    s = size * SS
    img = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    c = s / 2

    ring = g["ring"] * s
    stroke = max(SS, round(g["stroke"] * s))
    d.ellipse([c - ring, c - ring, c + ring, c + ring], outline=AMBER, width=stroke)

    dot = g["dot"] * s
    d.ellipse([c - dot, c - dot, c + dot, c + dot], fill=AMBER)

    # Ticks start just outside the ring's outer edge and run toward the corners
    # of the canvas, leaving a hair of margin so nothing is clipped.
    inner = ring + stroke / 2 + g["gap"] * s
    outer = min(inner + g["tick"] * s, c - stroke / 2)
    half = g["tickw"] * s / 2
    for dx, dy in ((0, -1), (0, 1), (-1, 0), (1, 0)):
        x0, y0 = c + dx * inner, c + dy * inner
        x1, y1 = c + dx * outer, c + dy * outer
        d.rectangle(
            [min(x0, x1) - (half if dx == 0 else 0), min(y0, y1) - (half if dy == 0 else 0),
             max(x0, x1) + (half if dx == 0 else 0), max(y0, y1) + (half if dy == 0 else 0)],
            fill=AMBER,
        )
    return img.resize((size, size), Image.LANCZOS)


if __name__ == "__main__":
    for size in sorted(GEOMETRY):
        draw(size).save(f"icons/icon{size}.png")
        print(f"icons/icon{size}.png")
