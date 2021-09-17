package interfaces

type IController interface {
	Run(workers int, stopCh chan struct{})
	OnUpdateImageString(url, tag, platformString, imageString string)
	GetReresourceName() string
}

type IImageNotifier interface {
	RegistImage(c IController, url, tag, platformString string)
	UnregistImage(c IController, url, tag, platformString string)
}

type IRemoteRegistry interface {
	GetImageString(url, tag, platformString string) (string, error)
}
