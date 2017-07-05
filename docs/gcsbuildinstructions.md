

# Download and install [Latest Golang package](https://golang.org/doc/install)

      // Downlad the latest Golang 
      ~$ sudo curl -O https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz
      ~$ tar -xvf go1.8.3.linux-amd64.tar.gz

     // To setup GO environment variables
     // add the following settings into ~/.profile before sourcing it
     export GOPATH=/home/yourusername/golang
     export GOROOT=/home/yourusername/go
     export PATH=$PATH:$GOROOT/bin

# Clone the opengcs repo to your local system

    // The "src/github.com/Microsoft" part of the path is required

    mkdir -p $GOPATH/src/github.com/Microsoft

    cd $GOPATH/src/github.com/Microsoft

    git clone https://github.com/Microsoft/opengcs.git opengcs

# Build GCS binaries

    // build gcs binaries
    cd $GOPATH/src/github.com/Microsoft/opengcs/service
    make
    
    // show all the built binaries
    ls bin

    On a successful build, you would get the following binaries
   createSandbox  exportSandbox  gcs  gcstools  tar2vhd  vhd2tar

