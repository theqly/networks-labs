#include <arpa/inet.h>
#include <iostream>
#include <memory>
#include <sys/socket.h>
#include <unistd.h>

class client {
private:
    int sock = -1;
    const std::string& server_ip;
    const int server_port;
public:
    client(const std::string& server_ip, const int server_port) :  server_ip(server_ip), server_port(server_port) { }

    void connect_to_server() {
        sock = socket(AF_INET, SOCK_STREAM, 0);

        if(sock == -1) {
            throw std::runtime_error("error in socket()");
        }

        sockaddr_in server_sockaddr_in{};
        server_sockaddr_in.sin_family = AF_INET;
        server_sockaddr_in.sin_port = htonl(server_port);
        inet_pton(AF_INET, server_ip.c_str(), &server_sockaddr_in.sin_addr);

        if(connect(sock, reinterpret_cast<sockaddr*>(&server_sockaddr_in), sizeof(server_sockaddr_in)) == -1) {
            throw std::runtime_error("error in connect()");
        }

        std::cout << "Connected to " << server_ip << ":" << server_port << std::endl;
    }

    ~client() {
        if(sock != -1) {
            close(sock);
        }
    }
};

int main(int argc, char** argv) {
    if(argc != 3) {
        std::cout << "Usage: " << argv[0] << " <PATH TO FILE> <SERVER IP>" << std::endl;
        return 1;
    }


    return  0;
}