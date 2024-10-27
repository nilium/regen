regen
=====

    $ go get go.spiff.io/regen

regen is a small tool to generate more or less random strings from Go RE2 regular expressions. You
can read up on RE2 at <https://github.com/google/re2/wiki/Syntax>.

As a few examples:

    $ regen -n 2 '0x[\da-f]{16}'
    0x8f5858102a5ce124
    0x3e4c9fee6c9f419d

    $ regen -n 3 '[a-z]{6,12}(\+[a-z]{6,12})?@[a-z]{6,16}(\.[a-z]{2,3}){1,2}'
    iprbph+gqastu@regegzqa.msp
    abxfcomj@uyzxrgj.kld.pp
    vzqdrmiz@ewdhsdzshvvxjk.pi

Essentially, all regen does is parse the regular expressions it's given and iterate over the tree
produced by [regexp/syntax](https://golang.org/pkg/regexp/syntax/) and attempt to generate strings
based on the ops described by its results. This could probably be optimized further by compiling the
resulting Regexp into a Prog, but I didn't feel like this was worthwhile when it's a very small
tool.

Currently, handling word boundaries is not supported and will cause regen to panic in response. The
way line endings and EOT is handled are also likely incorrect and they'll need some more thinking
put into them.

Some additional information can be found at <https://godoc.org/go.spiff.io/regen>.


Contributing
------------
Development of regen is frozen except for bug fixes since it does what I wanted it to do (for the
most part). My suggestion is to fork it if you have any plans to do more with it, a few others have
done this (e.g., to turn it into a testing package) to continue it in other directions and that's
perfectly fine.


License
-------
regen is licensed under a 2-clause BSD license. This can be found in LICENSE.txt.
