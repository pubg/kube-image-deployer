package interfaces

type IController interface {
	Run(workers int, stopCh chan struct{})
	OnUpdateImageString(url, tag, imageString string)
	GetReresourceName() string
}

type IImageNotifier interface {
	RegistImage(c IController, url, tag string)
	UnregistImage(c IController, url, tag string)
}

type IRemoteRegistry interface {
	GetImageString(url, tag string) (string, error)
}
