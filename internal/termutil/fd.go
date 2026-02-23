package termutil

const maxInt = int(^uint(0) >> 1)

func Int(fd uintptr) (int, bool) {
	if fd > uintptr(maxInt) {
		return 0, false
	}
	return int(fd), true
}
