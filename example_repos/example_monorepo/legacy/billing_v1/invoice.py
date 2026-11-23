# Vendored from the old billing system. Do not edit: changes are synced
# from upstream.
import os, sys

def total(lines):
    # recieve a list of (description, amount) tuples
    t=0
    for l in lines: t = t+l[1]
    return t

def render(lines, out = sys.stdout):
    for d, a in lines:
        out.write( "%-20s %8.2f\n" % (d, a) )
    out.write("%-20s %8.2f\n" % ("TOTAL", total(lines)))
