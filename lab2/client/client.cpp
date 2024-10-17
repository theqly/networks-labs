#include <cstring>
#include <arpa/inet.h>
#include <iostream>
#include <fstream>
#include <sys/socket.h>
#include <unistd.h>

class client {
private:
    const int BUFFER_SIZE = 1024;

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
        server_sockaddr_in.sin_port = htons(server_port);
        inet_pton(AF_INET, server_ip.c_str(), &server_sockaddr_in.sin_addr);

        std::cout << server_ip.c_str() << std::endl;

        if(connect(sock, reinterpret_cast<sockaddr*>(&server_sockaddr_in), sizeof(server_sockaddr_in)) == -1) {
            throw std::runtime_error("error in connect()");
        }

        std::cout << "connected to " << server_ip << ":" << server_port << std::endl;
    }

    void send_file(const std::string& file_path) {

        // PROTOCOL:
        // 1. send filename len
        // 2. send filename
        // 3. send size of file
        // 4. send (size of file) bytes of file
        // 5. recv response that all (size of file) are received

        std::ifstream file(file_path, std::ios::binary);

        if(!file.is_open()) {
            throw std::runtime_error("error in opening file with path " + file_path);
        }

        file.seekg(0, std::ios::end);
        const uint64_t file_size = file.tellg();
        file.seekg(0, std::ios::beg);

        const uint16_t file_path_len = htons(file_path.size());

        std::cout << "opened a file and trying to send" << std::endl;

        if(send(sock, &file_path_len, sizeof(file_path_len), 0) == -1) {
            throw std::runtime_error("error in send(file path len)");
        }

        std::cout << "sent a file path len: " << file_path_len << std::endl;

        if(send(sock, file_path.c_str(), file_path.size(), 0) == -1) {
            throw std::runtime_error("error in send(filename)");
        }

        std::cout << "sent a file path: " << file_path << std::endl;

        const uint64_t n_file_size = htonll(file_size);

        if(send(sock, &n_file_size, sizeof(n_file_size), 0) == -1) {
            throw std::runtime_error("error in send(file size)");
        }

        std::cout << "sent a file size: " << file_size << std::endl;

        char buffer[BUFFER_SIZE];
        uint64_t sent_sum = 0;

        while(sent_sum < file_size) {
            file.read(buffer, BUFFER_SIZE);
            auto read = file.gcount();

            if(read > 0) {
                ssize_t sent = send(sock, buffer, read, 0);
                if(sent == -1){
                    throw std::runtime_error("error in send()");
                }
                sent_sum += sent;
            }

            if(file.eof()) break;
        }

        std::cout << "after cycle" << std::endl;

        file.close();

        char response[128];

        if(recv(sock, response, sizeof(response), 0) == -1){
            throw std::runtime_error("error in recv()");
        }

        std::cout << "response from server: " << response << std::endl;
    }

    static uint64_t htonll(uint64_t value) {
        return ((uint64_t)htonl(value & 0xFFFFFFFF) << 32) | htonl(value >> 32);
    }

    ~client() {
        if(sock != -1) {
            close(sock);
        }
    }

};

int main(int argc, char** argv) {
    if(argc != 4) {
        std::cerr << "Usage: " << argv[0] << " <PATH TO FILE> <SERVER IP> <SERVER PORT>" << std::endl;
        return 1;
    }

    std::string file_path = argv[1];
    std::string server_ip = argv[2];
    const int server_port = std::stoi(argv[3]);

    client _client(server_ip, server_port);

    try {
        _client.connect_to_server();
        _client.send_file(file_path);
    } catch (const std::exception& e){
        std::cerr << "catch exception: " << e.what() << std::endl;
        return 1;
    }

    return  0;
}