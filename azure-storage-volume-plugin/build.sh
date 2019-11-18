set -e

export pluginName=docker-volume-plugin-azure-storage

docker plugin rm -f evops/$pluginName || true
docker rmi -f rootfsimage || true
docker build -t rootfsimage .. -f Dockerfile
id=$(docker create rootfsimage true) # id was cd851ce43a403 when the image was created
rm -rf build/rootfs
mkdir -p build/rootfs
docker export "$id" | tar -x -C build/rootfs
docker rm -vf "$id"
cp ./config.json build
if [ -z "$TAG" ]
then
    docker plugin create evops/$pluginName build
    docker plugin push evops/$pluginName
else
    docker plugin create evops/$pluginName:$TAG build
    docker plugin push evops/$pluginName:$TAG
fi