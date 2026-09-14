package proxy

import "errors"

var errInvalidTarget = errors.New("proxy: target must be an absolute URL with scheme and host")
