import ast
import sys

status = 0
for path in sys.argv[1:]:
    with open(path) as f:
        source = f.read()
    try:
        ast.parse(source, path)
    except SyntaxError as e:
        print("%s:%d: syntax error: %s" % (path, e.lineno, e.msg))
        status = 1
sys.exit(status)
