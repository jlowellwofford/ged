# ged
`ged` is `ed` rewritten in pure Go.  Ged is the wizard of earthsea.

`ged` is not yet complete.

The following has been implemented:
- Full line address parsing (including RE and markings)
- Implmented commands:
  - q
  - Q
  - d
  - p
  - n
  - h
  - H
  - a
  - i
  - c
  - w
  - W
  - k
  - e
  - E
  - f
  - =
  - \#
  - l
  - r
  - j
  - m
  - t
  - y
  - x
  - P
  - s
  - u
  - z
  - !

The following has *not* yet been implemented:
- Unimplemented commands:
  - g
  - G
  - v
  - V

- Commands that should interpret ! but don't:
  - e
  - r
  - w