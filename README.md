# houdini: the world's worst containerizer

Houdini is effectively a no-op Garden backend portable to esoteric platforms
like Darwin and Windows.

For a Linux Garden backend, please use
[Guardian](https://github.com/cloudfoundry/guardian) instead.

Houdini makes no attempt to isolate containers from each other. It puts them
in a working directory, and hopes for the best.

On Windows, it will at least use [Job
Objects](https://msdn.microsoft.com/en-us/library/windows/desktop/ms684161%28v=vs.85%29.aspx)
to ensure processes are fully cleaned up. On OS X, there are basically no
good ways to do this, so it doesn't bother.
