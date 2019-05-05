# DMKHunter

**This package is currently experimental! The api and usage could be change!**

The goal of this application is to provide some additional security features to any 
application which needs to be run on a shared hosting package.

If you have the ability to install packages, we strongly recommend to
use `rkhunter` and `clamav` directly!

## Usage

Download the application from the releases page to any folder. Then you must
create a file `.hunter.conf`.

```
$ dmkhunter rescan # If you set up a database analyzer
$ dmkhunter analyze
```