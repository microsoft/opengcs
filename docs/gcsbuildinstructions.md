

# Download and install [Latest Golang package](https://golang.org/doc/install)

    eg:

      // Downlad the latest Golang 

      ~$ sudo curl -O https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz
      ~$ tar -xvf go1.6.linux-amd64.tar.gz

     // setup GO environment variables
     //add the following settings into ~/.profile before sourcing it
    export GOPATH=/home/yourusername/golang
    export GOROOT=/home/yourusername/go
    export PATH=$PATH:$GOROOT/bin

# Clone the opengcs repro to your local system

    // src/github.com/Microsoft par of the path is required
    mkdir -p $GOPATH/src/github.com/Microsoft
    cd $GOPATH/src/github.com/Microsoft
    git clone https://github.com/Microsoft/opengcs.git opengcs

# Build GCS binaries

    // build gcs binaries
    cd opengcs/service
    make
    
    // show all the built binaries
    ls bin

# Open Guest Compute Service (OpenGCS)

Open Guest Compute Service (OpenGCS) is an open source project to further the development of a production quality implementation of Linux Hyper-V container on Windows. 

Getting Started

How to build GCS binarie [How to build GCS binaries](https://github.com/Microsoft/opengcs/gcsbuildinstructions/)

# Contributing

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
