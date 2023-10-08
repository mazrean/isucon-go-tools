package isudb

var (
	enableRetry          = false
	enableQueryTrace     = true
	fixInterpolateParams = true
)

func SetRetry(enable bool) {
	enableRetry = enable
}

func SetQueryTrace(enable bool) {
	enableQueryTrace = enable
}

func SetFixInterpolateParams(enable bool) {
	fixInterpolateParams = enable
}
