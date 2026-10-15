import json
import sys

status = 0
for path in sys.argv[1:]:
    try:
        with open(path) as f:
            json.load(f)
    except ValueError as e:
        print("%s: invalid json: %s" % (path, e))
        status = 1
sys.exit(status)
