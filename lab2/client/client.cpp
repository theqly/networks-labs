#include <cstring>
#include <arpa/inet.h>
#include <iostream>
#include <fstream>
#include <memory>
#include <sys/socket.h>
#include <unistd.h>

class client {
private:
    const uint16_t BUFFER_SIZE = 1024 * 1024;

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

    void send_file(const std::string& file_path) {
        std::ifstream file(file_path, std::ios::binary);

        if(!file.is_open()) {
            throw std::runtime_error("error in opening file with path " + file_path);
        }

        file.seekg(0, std::ios::end);
        const uint64_t file_size = file.tellg();
        file.seekg(0, std::ios::beg);

        const char* filename = strrchr(file_path.c_str(), '/');
        if(filename == nullptr) filename = file_path.c_str();
        else filename++;

        // PROTOCOL:
        // 1. send filename
        // 2. send size of file
        // 3. send (size of file) bytes of file

        if(send(sock, filename, strlen(filename), 0) == -1) {
            throw std::runtime_error("error in send(filename)");
        }

        if(send(sock, &file_size, sizeof(file_size), 0) == -1) {
            throw std::runtime_error("error in send(file size)");
        }

        char buffer[BUFFER_SIZE];
        uint64_t total_send = 0;
        while(total_send < file_size) {
            file.read(buffer, BUFFER_SIZE);
            auto readed = file.gcount();

            if(readed > 0) {
                
            }
        }


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