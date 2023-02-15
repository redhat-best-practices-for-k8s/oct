# TNF & OCT integration roadmap/tasks proposal

## Roadmap proposal
* Move the offline api implementation functions into OCT repo so TNF can use import and use them. TNF's can remain but the offline implementation should use OCT's functions.

## TNF
### Tasks

1. Remove TNF's offline function implementations and use OCTs ones.

## OCT
### Tasks
1. Move TNF's certdb api (offline functions implementations) to OCT.
2. Improve OCT's fetch tool. There'are a lot of improvements that can be done in the existing TNF code for the fetch tool. To name some of them:
    - Operators are currently downloading every field from the catalog, but we only need some of them. Also, they're saved on disk in a file for each http response. We can merge everything into a single file as with the containers.
    - The size of this db files will increase monotonically, as new entries are added every day. Maybe saving everything in json format (either in single or in multiple files) is not the best way to go for future-proof. We might need to put a size limit on this files. SQLite can be explored as an alternative here.
    - We download sequentially, page by page, but we could issue several queries in parallel (using go-routines).

### Optionally
1. Separate OCT repo in two repositories? Is it really worth it?
    1. oct : just the fetch app.
    2. oct-api : api to be used by both OCT and TNF.

