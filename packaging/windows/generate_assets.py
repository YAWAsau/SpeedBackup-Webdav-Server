"""Draw the original vector-like installer artwork at multiple pixel sizes."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).with_name("assets")
OUT.mkdir(exist_ok=True)
mark = Image.new("RGBA", (256, 256), (0, 0, 0, 0))
d = ImageDraw.Draw(mark)
d.rounded_rectangle((4, 4, 252, 252), radius=60, fill="#16384B")
for y in (57, 111, 165):
    d.rounded_rectangle((55, y, 201, y + 35), radius=9, fill="#D8EFEE")
    d.ellipse((171, y + 11, 184, y + 24), fill="#219C8D")
mark.save(OUT / "mark.png")
mark.save(OUT / "server.ico", sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
im = Image.new("RGB", (656, 1256), "#102A3A")
d = ImageDraw.Draw(im)
for y in range(im.height):
    t = y / im.height
    d.line((0, y, im.width, y), fill=(16 + int(7*t), 42 + int(20*t), 58 + int(16*t)))
for x, y, r in [(560, 1100, 400), (480, 1160, 260)]:
    d.ellipse((x-r, y-r, x+r, y+r), outline="#315C68", width=3)
im.paste(mark, (64, 116), mark)
font = ImageFont.truetype("C:/Windows/Fonts/segoeui.ttf", 50)
small = ImageFont.truetype("C:/Windows/Fonts/segoeui.ttf", 28)
d.text((64, 430), "SpeedBackup", font=font, fill="#F0F8F8")
d.text((64, 497), "SERVER", font=small, fill="#7BD6C5")
d.line((64, 583, 160, 583), fill="#7BD6C5", width=5)
d.text((64, 648), "Your backups.", font=small, fill="#D3E6EC")
d.text((64, 697), "Your storage.", font=small, fill="#D3E6EC")
d.text((64, 1120), "SET UP  /  CONNECT  /  PROTECT", font=ImageFont.truetype("C:/Windows/Fonts/segoeui.ttf", 18), fill="#A4C9D0")
im.save(OUT / "welcome.png")
