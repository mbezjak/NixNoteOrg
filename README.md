# NixNoteOrg

This is fork of [EverOrg](https://github.com/mgmart/EverOrg) project repurposed
to deal with [NixNote2](https://github.com/baumgarr/nixnote2). If you are on
Linux this might be more useful than the original EverOrg project because there
are no official Evernote desktop client on Linux. Unofficial client is NixNote2
that has slightly different export format (nnex). The good news is that you can
automate the steps as a simple non interactive script.

## Prerequisites

1. [NixNote2](http://www.nixnote.org/) should be installed and logged in to Evernote.
2. go compiler should be in your $PATH

## How to use

Here is example steps that you can copy/paste into a bash script and customize
as you see fit.

    mkdir out
    nixnote2 sync
    nixnote2 export --search='*' --output=out/all.nnex
    go build
    ./NixNoteOrg -input out/all.nnex

## License

NixNoteOrg is distributed under GNU Public License version 3. See LICENSE.md for
more info.
