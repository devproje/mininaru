# Copyright and licensing

## Software

Copyright (C) 2026 Wonhyeok Kim (Project_IO)

mininaru is free software: you can redistribute it and/or modify it under the
terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. The full text is in [LICENSE](LICENSE).

SPDX identifier: `GPL-3.0-or-later`

This covers the source code, the build files, and the documentation in this
repository, with the exception below.

## Artwork — all rights reserved

`assets/logo.png`, the naru character, and any other artwork or likeness of the
character in this repository are **not** covered by the GNU General Public
License. They are:

> Copyright (C) 2026 Wonhyeok Kim (Project_IO). All rights reserved.

You may redistribute them verbatim as part of an unmodified copy of this
repository, and use them to refer to this project. Any other use — including
modification, redistribution on their own, use as your own branding, or
commercial use — requires written permission from the copyright holder.

Removing the artwork does not affect your rights to the software: nothing in
the program depends on it, and the GPL applies to everything else regardless.

## Third-party components

The compiled binary links libraries under the MIT, BSD 2-Clause, BSD 3-Clause,
and Apache-2.0 licenses. Apache-2.0 is compatible with version 3 of the GNU
General Public License but not with version 2, which is why this project is
licensed as `GPL-3.0-or-later` rather than `GPL-2.0-only`.

Their terms and copyright notices travel with the modules themselves; run
`go list -m all` to enumerate them, and see each module's own LICENSE file.
