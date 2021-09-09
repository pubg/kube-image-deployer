package interfaces

type IController interface {
	Run(workers int, stopCh chan struct{})
	OnUpdateImageHash(url, tag, imageHash string)
	GetReresourceName() string
}

type IImageNotifier interface {
	RegistImage(c IController, url, tag string)
	UnregistImage(c IController, url, tag string)
}

type IRemoteRegistry interface {
	GetImageHash(url, tag string) (string, error)
}
