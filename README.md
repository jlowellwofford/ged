# ged
`ged` is `ed` rewritten in pure Go.  Ged is the wizard of earthsea.

## Using `ged`

To install `ged`, you need Go >= 1.12.  Once you have that, just:

```console
$ go get github.com/jlowellwofford/ged
```

This will install `ged` in `$GOPATH/bin`.

To build by hand, use (inside the folder where you have the `ged` source):

```
$ git clone https://github.com/jlowellwofford/ged
$ cd ged
$ go build . 
```

Once you have `ged` in your path, you should be able to run it and it should behave more-or-less like `GNU Ed`.

## About `ged`

`ged` is intended to be a feature-complete mimick of [GNU Ed](https://www.gnu.org/software/ed//).  It is a close enough mimick that the [GNU Ed Man Page](https://www.gnu.org/software/ed/manual/ed_manual.html) should be a reliable source of documentation.  Divergence from the man page is generally considered a bug (unless it's an added feature).

There are a few known differences:

- `ged` uses `go`'s `regexp` package, and as such may have a somewhat different regular expression syntax.  Note, however, that backreferences follow the `ed` syntax of `\<ref>`, not the `go` syntax of `$<ref>`.
- there has been little/no attempt to make particulars like error messages match `GNU Ed`. 
- rather than being an error, the 'g' option for 's' simply overrides any specified count.

The following has been implemented:
- Full line address parsing (including RE and markings)
- Implmented commands: !, #, =, E, H, P, Q, W, a, c, d, e, f, h, i, j, k, l, m, n, p, q, r, s, t, u, w, x, y, z

The following has *not* yet been implemented:
- Unimplemented commands: g, G, v, V

- Commands that should interpret ! but don't: w