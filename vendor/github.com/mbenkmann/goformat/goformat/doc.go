// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Gofmt formats Go programs.
It uses tabs for indentation and blanks for alignment.
Alignment assumes that an editor is using a fixed-width font.

Without an explicit path, it processes the standard input.  Given a file,
it operates on that file; given a directory, it operates on all .go files in
that directory, recursively.  (Files starting with a period are ignored.)
By default, gofmt prints the reformatted sources to standard output.

Usage:
	gofmt [flags] [path ...]

The flags are:
	-d
		Do not print reformatted sources to standard output.
		If a file's formatting is different than gofmt's, print diffs
		to standard output.
	-e
		Print all (including spurious) errors.
	-l
		Do not print reformatted sources to standard output.
		If a file's formatting is different from gofmt's, print its name
		to standard output.
	-r rule
		Apply the rewrite rule to the source before reformatting.
	-s
		Try to simplify code (after applying the rewrite rule, if any).
	-w
		Do not print reformatted sources to standard output.
		If a file's formatting is different from gofmt's, overwrite it
		with gofmt's version. If an error occurred during overwriting,
		the original file is restored from an automatic backup.

Debugging support:
	-cpuprofile filename
		Write cpu profile to the specified file.


The rewrite rule specified with the -r flag must be a string of the form:

	pattern -> replacement

Both pattern and replacement must be valid Go expressions.
In the pattern, single-character lowercase identifiers serve as
wildcards matching arbitrary sub-expressions; those expressions
will be substituted for the same identifiers in the replacement.

When gofmt reads from standard input, it accepts either a full Go program
or a program fragment.  A program fragment must be a syntactically
valid declaration list, statement list, or expression.  When formatting
such a fragment, gofmt preserves leading indentation as well as leading
and trailing spaces, so that individual sections of a Go program can be
formatted by piping them through gofmt.

Examples

To check files for unnecessary parentheses:

	gofmt -r '(a) -> a' -l *.go

To remove the parentheses:

	gofmt -r '(a) -> a' -w *.go

To convert the package tree from explicit slice upper bounds to implicit ones:

	gofmt -r 'α[β:len(α)] -> α[β:]' -w $GOROOT/src

The simplify command

When invoked with -s gofmt will make the following source transformations where possible.

	An array, slice, or map composite literal of the form:
		[]T{T{}, T{}}
	will be simplified to:
		[]T{{}, {}}

	A slice expression of the form:
		s[a:len(s)]
	will be simplified to:
		s[a:]

	A range of the form:
		for x, _ = range v {...}
	will be simplified to:
		for x = range v {...}

	A range of the form:
		for _ = range v {...}
	will be simplified to:
		for range v {...}

This may result in changes that are incompatible with earlier versions of Go.
*/
package main

const usage_text = `GOFORMAT
========

NAME
----
goformat - Alternative to gofmt with configurable formatting style (indentation etc.)

INSTALLATION
------------
    go get winterdrache.de/goformat/goformat
  
or
  
     go get github.com/mbenkmann/goformat/goformat
 
Either command will install the goformat binary to $GOPATH/bin.
Alternatively, clone the git repository and execute 'make' at the top level.
This will create the goformat binary in the bin/ subdirectory of the cloned repository.

SYNOPSIS
--------

    goformat [options]            # read from stdin; code fragment allowed
    goformat [options] file ...   # single file; must be a complete source file
    goformat [options] dir ...    # recursive directory traversal (only files with ".go" extension)

DESCRIPTION
-----------
goformat is a code auto-formatter/indenter/pretty-printer/beautifier like gofmt.
If used with the same command line arguments, it will produce the exact same
output as gofmt. However unlike gofmt it allows you to customize the style (in
particular the indentation) it uses to format code.

OPTIONS
--------

###  -cpuprofile file
Write CPU profile to this file (for performance analysis with go tool pprof).
        
###  -d
Output diff between original and reformatted code to stdout. May be combined with -w,
so that the reformatted code is written to the original file and the diff to stdout.

###  -e
Report all parse errors. By default only the first 10 errors on different lines are reported.

###  -fragment
When operating on files/directories, goformat by default expects them to be
complete go source files (with package keyword etc.) and reports incomplete
code as errors. With this switch, goformat accepts partial code fragments,
e.g. a single function or if-block.
The code still needs to be "somewhat" complete. If you could paste the fragment
either after the package line or into a function of a proper Go source file,
then the fragment is acceptable.

When processing a code fragment, goformat will attempt to add back the initial
indentation of the code fragment after doing its formatting, so that a code
block cut from a larger piece of code can be formatted and then pasted at the
original position and the indentation will fit.

This operation mode is default when processing input from stdin.

###  -l
List files whose formatting differs from goformat's on stdout.
May be combined with -w so that the reformatted code is written to the original
file and then list of changed files is printed to stdout.

###  -r "rule"
Apply rewrite rule to code. See section below for an explanation of rewrite rules.
        
###  -s
Simplify code without changing its semantics.
See the section below for a list of the rules applied.

### -style file
### -style "code"
Change the formatting rules. You can pass either the path of a file containing
the style description or the style description itself. In the latter case make
sure you use appropriate shell quoting to escape whitespace.
See the section below for a description of the style language.

###  -w
Overwrite source file(s) with formatted result instead of printing the code to
standard output. Files with parse errors (in particular files that are not actually
Go source code) will not be modified. Note that when traversing directories
goformat will only look at files with ".go" extension.

REWRITE RULES
-------------
The -r option specifies a rewrite rule of the form

    pattern -> replacement
    
where both pattern and replacement are valid Go expressions.
In the pattern, single-character lowercase identifiers serve as wildcards
matching arbitrary sub-expressions, and those expressions are substituted
for the same identifiers in the replacement.

Don't forget to quote the rule to prevent the shell from messing with it.

### Examples
    gofmt -r 'bytes.Compare(a, b) == 0 -> bytes.Equal(a, b)'
    gofmt -r 'bytes.Compare(a, b) != 0 -> !bytes.Equal(a, b)'
    gofmt -r 'a[b:len(a)] -> a[b:]'

CODE SIMPLIFICATION
-------------------
When the -s option is used, the following simplifications will be performed on the code:

An array, slice, or map composite literal of the form:

    []T{T{}, T{}}

will be simplified to:

    []T{{}, {}}

A slice expression of the form:

    s[a:len(s)]

will be simplified to:

    s[a:]

A range of the form:

    for x, _ = range v {...}
    
will be simplified to:

    for x = range v {...}

A range of the form:

    for _ = range v {...}

will be simplified to:
    
    for range v {...}

*This may result in changes that are incompatible with earlier versions of Go.*

STYLE LANGUAGE
--------------
To specify the default style that is used whenever no more specific style rule is
applicable, simply write the style settings separated by whitespace, e.g.

    indent=tab shift=2

To specify style options that should only be applied within a certain
context, write one or more context specifiers followed by the style options,
e.g.

    indent=tab            # default
    comment shift=2       # shift comments right by 2 spaces
    head comment shift=0  # except for comments before the "package" keyword

Rules that occur later in the style file and affect the same context will
override earlier rules.

Everything following a "#" or "//" until the next line break is a comment,
e.g.

    # This is a comment.
    // This is a comment, too.


### Style options
    
    indent=tab

Every level of indentation is a single tab character. Independent of this
setting, vertical column alignment is always done with spaces.

    indent=keep

Keep the original indentation. If linebreaks are introduced, the indentation
from the broken line will be used. For block comments, the closing `+"`*/`"+` will
always be indented the same way as the opening `+"`"+`/*`+"`"+`. Lines consisting only
of whitespace will not be preserved, even with this setting.

    indent=<number>

Every level of indentation is done with this number of spaces. No tabs will
be used. 0 is permitted and will result in left-aligned code. Negative
values are illegal.

    pad=<number>

When doing vertical/column alignment, add this number of spaces at the
right side of each column. This will most prominently affect // comments on the
same line as code. E.g.

    a := 1     // Magic number
    b := 200   // Another magic number
            ^^^
           pad=3

Note that padding is only relevant for the longest column. Shorter columns
will get more spaces because of vertical alignment.

    shift=<number>

Add this number of spaces after the initial indentation, effectively
shifting the text to the right. Mostly useful for comments.

    column=<number>

When doing column alignment, each column will be padded to at least this
number of characters. Note that pad=... is applied first, so column=... only
changes columns that are too short even with pad=...

    enter=<number>

After the opening brace of a block, this number of indent steps (each of
the size specified with indent=...) will be added to the indentation. Note
that the default is 0 for some contexts (e.g. switch) but 1 for most others.
There is no global enter=... value corresponding to the default behaviour.

	inlineblocks=keep

If a {...} or case block in the input has both braces on the same line, do not
insert newlines after/before the braces/case. E.g.

	if a == b { return } else { break }

With inlineblocks=keep this would stay on a single line.

	inlineblocks=never

This is the default behaviour which forces newlines after "case:", "default:",
and "{" and before "}".
          
### Context specifiers

    head

This context specifier refers to everything up to and including the
"package" keyword.

    comment

applies to any comment, line or block.

    comment[line]

applies only to line comments, i.e. //...

    comment[block]
   
applies only to block comments, i.e. /*...*/

    switch

applies to switch and type switch statements.

    case

applies to case clauses (including the default case)

	select

applies to select statements.

AUTHOR
------
The Go Authors (original gofmt code goformat is based on)

Matthias S. Benkmann, <msb@winterdrache.de> (style configurability)

BUGS, FEATURE REQUESTS
----------------------
Please use the issue tracker at https://github.com/mbenkmann/goformat/issues
for bug reports and feature requests.

`

// BUG(rsc): The implementation of -r is a bit slow.
// BUG(gri): If -w fails, the restored original file may not have some of the
// original file attributes.
