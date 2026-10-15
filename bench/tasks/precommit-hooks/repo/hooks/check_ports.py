import sys

status = 0
for path in sys.argv[1:]:
    previous = ""
    ports = {}
    count = 0
    with open(path) as f:
        for number, line in enumerate(f, 1):
            line = line.rstrip("\n")
            if not line:
                continue
            count += 1
            name, _, value = line.partition(":")
            name, value = name.strip(), value.strip()
            print("%s:%d: checking %s (port %s)" % (path, number, name, value))
            if not value.isdigit():
                print("%s:%d: error: port of %s is not a number: %r" % (path, number, name, value))
                status = 1
            elif value in ports:
                print("%s:%d: error: port %s of %s is already used by %s" % (path, number, value, name, ports[value]))
                status = 1
            else:
                ports[value] = name
            if name < previous:
                print("%s:%d: error: %s is out of order, it sorts before %s" % (path, number, name, previous))
                status = 1
            previous = name
    print("%s: %d services checked" % (path, count))
sys.exit(status)
