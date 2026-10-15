import sys

status = 0
for path in sys.argv[1:]:
    with open(path) as f:
        lines = f.read().split("\n")
    fixed = [line.rstrip(" \t") for line in lines]
    if fixed != lines:
        with open(path, "w") as f:
            f.write("\n".join(fixed))
        print("Fixing " + path)
        status = 1
sys.exit(status)
