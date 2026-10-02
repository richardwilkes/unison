# Changes since the last release

## New & Improved

- Added `FilesRequestedAtLaunch()`, which can be called before `Start()` to obtain the files the operating system asked
  the application to open when it launched it. On macOS, documents opened from the Finder (double-clicked, chosen via
  "Open With", or dropped on the Dock icon) arrive as an Apple Event rather than on the command line, so an application
  that hands its files off to an already-running instance before calling `Start()` can now include them. The files are
  still delivered to the `OpenFilesCallback` if `Start()` is called afterward.

## Bug Fixes

- (none yet)
