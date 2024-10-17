#include <arpa/inet.h>
#include <cstring>
#include <iostream>
#include <sys/socket.h>
#include <unistd.h>
#include <thread>
#include <fstream>

class server {
private:
    const int MAX_FILE_PATH_LEN = 4096;
    const int BUFFER_SIZE = 1024;

    int sock = -1;
    const int port;

    int number_of_clients;

public:
    explicit server(const int port) : port(port), number_of_clients(0) { }

    void run() {
        sock = socket(AF_INET, SOCK_STREAM, 0);

        if(sock == -1) {
            throw std::runtime_error("error in socket()");
        }

        sockaddr_in server_sockaddr_in{};
        server_sockaddr_in.sin_family = AF_INET;
        server_sockaddr_in.sin_port = htons(port);
        server_sockaddr_in.sin_addr.s_addr = INADDR_ANY;

        if(bind(sock, reinterpret_cast<sockaddr*>(&server_sockaddr_in), sizeof(server_sockaddr_in)) == -1) {
            throw std::runtime_error("error in bind()");
        }

        if(listen(sock, 5) == -1) {
            throw std::runtime_error("error in listen()");
        }

        std::cout << "server started on port " << port << std::endl;

        while(true) {
            sockaddr_in client_sockaddr_in{};
            socklen_t len = sizeof(client_sockaddr_in);
            const int client_sock = accept(sock, reinterpret_cast<sockaddr*>(&client_sockaddr_in), &len);

            if (client_sock == -1) {
                std::cerr << "error in accept()" << std::endl;
                continue;
            }

            std::thread(&server::handle_client, this, client_sock, number_of_clients).detach();

            number_of_clients++;
        }

    }

    static uint64_t ntohll(uint64_t value) {
        return ((uint64_t)ntohl(value & 0xFFFFFFFF) << 32) | ntohl(value >> 32);
    }

    ~server() {
        if(sock != -1) {
            close(sock);
        }
    }

private:
    void handle_client(const int client_sock, const int client_number) {
        std::cout << "*** new connection ***" << std::endl;

        char file_path[MAX_FILE_PATH_LEN + 1];
        uint16_t file_path_len;

        if(recv(client_sock, &file_path_len, sizeof(file_path_len), MSG_WAITALL) <= 0) {
            std::cerr << "error in recv(file path len)" << std::endl;
            close(client_sock);
            return;
        }

        file_path_len = ntohs(file_path_len);

        if(recv(client_sock, file_path, file_path_len, MSG_WAITALL) <= 0) {
            std::cerr << "error in recv(file path)" << std::endl;
            close(client_sock);
            return;
        }

        const char* filename = strrchr(file_path, '/');
        if(filename == nullptr) filename = file_path;
        else filename++;

        std::cout << "received filename: " << filename << std::endl;

        uint64_t file_size;
        if(recv(client_sock, &file_size, sizeof(file_size), MSG_WAITALL) <= 0) {
            std::cerr << "error in recv(file size)" << std::endl;
            close(client_sock);
            return;
        }

        file_size = ntohll(file_size);

        std::cout << "received file size: " << file_size << std::endl;

        std::ofstream file("uploads/" + std::string(filename), std::ios::binary);
        if(!file.is_open()) {
            std::cerr << "error in opening file with path: " << "uploads/" + std::string(filename) << std::endl;
            close(client_sock);
            return;
        }
    } catch

        try {
            char buffer[BUFFER_SIZE];
            uint64_t overall_received = 0;
            uint64_t last_received = 0;

            auto start_time = std::chrono::steady_clock::now();
            auto last_time = std::chrono::steady_clock::now();

            while(overall_received < file_size) {

                ssize_t received = recv(client_sock, buffer, sizeof(buffer), 0);

                if(received <= 0) break;

                file.write(buffer, received);
                overall_received += received;
                last_received += received;

                auto current_time = std::chrono::steady_clock::now();

                std::chrono::duration<double> brk = current_time - last_time;

                if(brk.count() >= 3.0) {

                    std::chrono::duration<double> general_brk = current_time - start_time;

                    double instant_speed = ((double)last_received / 1024.0) / brk.count();
                    double average_speed = ((double)overall_received / 1024.0) / general_brk.count();

                    std::cout << "*** client " << client_number << " *** " << "instant speed: " << instant_speed << " kb/s, average speed: " << average_speed << " kb/s"<< std::endl;

                    last_time = current_time;
                    last_received = 0;
                }
            }

            auto overall_time = std::chrono::duration<double>(std::chrono::steady_clock::now() - start_time);

            std::cout << "*** client " << client_number << " *** " << "speed: " << ((double)overall_received / 1024.0) / overall_time.count() << " kb/s" << std::endl;

            std::cout << "after cycle" << std::endl;

            if(overall_received == file_size) {
                if(send(client_sock, "all right", sizeof("all right"), 0) == -1) {
                    throw std::runtime_error("error in send()");
                }
            } else {
                if(send(client_sock, "all bad", sizeof("all bad"), 0) == -1) {
                    throw std::runtime_error("error in send()");
                }
            }

            file.close();
            close(client_sock);
        } catch (const std::exception& e) {
            file.close();
            close(client_sock);
        }
    }
};



int main(int argc, char** argv) {
    if(argc != 2) {
        std::cerr << "Usage: " << argv[0] << " <PORT>" << std::endl;
        return 1;
    }

    const int port = std::stoi(argv[1]);

    server _server(port);

    try{
        _server.run();
    } catch (const std::exception& e) {
        std::cerr << "catch exception: " << e.what() << std::endl;
    }


    return  0;
}

