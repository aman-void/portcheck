package main

// Windows has no Unix SIGPIPE delivery; failed writes return ordinary errors.
func prepareSignals() {}
