// Determine stream of a package.
package db

func UpTrace(p Pkg) (urls []string) {
	switch p.Repo.Type {
	case Rpm:
		return rpmTrace(p)
	}
	l.DPanic("unreachable in uptrace")
	return
}

func rpmTrace(p Pkg) (urls []string) {
	// p.Meta
	return
}
