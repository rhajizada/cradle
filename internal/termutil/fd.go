package termutil

const MaxInt = int(^uint(0) >> 1)

func Int(fd uintptr) (int, bool) {
	if fd > uintptr(MaxInt) {
		return 0, false
	}
	return int(fd), true
}
