package main

import "errors"

var ErrNotFound = errors.New("not found")
var ErrInvalidPath = errors.New("invalid path")
var ErrInvalidHash = errors.New("hash mismatch")
var ErrLocked = errors.New("already locked")
var ErrUnlocked = errors.New("not locked")
var ErrNotChanged = errors.New("not changed")
