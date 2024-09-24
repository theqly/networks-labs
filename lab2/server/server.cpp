#include <arpa/inet.h>
#include <iostream>
#include <sys/socket.h>
#include <unistd.h>

class server {
private:
    int sock = -1;
    const int port;

public:
    explicit server(const int port) : port(port) { }

    void run() {
        sock = socket(AF_INET, SOCK_STREAM, 0);

        if(sock == -1) {
            throw std::runtime_error("error in socket()");
        }

        sockaddr_in server_sockaddr_in{};
        server_sockaddr_in.sin_family = AF_INET;
        server_sockaddr_in.sin_port = htonl(port);
        server_sockaddr_in.sin_addr.s_addr = INADDR_ANY;

        if(bind(sock, reinterpret_cast<sockaddr*>(&server_sockaddr_in), sizeof(server_sockaddr_in)) == -1) {
            throw std::runtime_error("error in bind()");
        }

        if(listen(sock, 5) == -1) {
            throw std::runtime_error("error in listen()");
        }

        std::cout << "Server started on port " << port << std::endl;

        while(true) {
            sockaddr_in client_sockaddr_in{};
            socklen_t len = sizeof(client_sockaddr_in);
            const int client_sock = accept(sock, reinterpret_cast<sockaddr*>(&client_sockaddr_in), &len);

            if (client_sock == -1) {
                std::cout << "error in accept()" << std::endl;
                continue;
            }
            handle_client(client_sock);
        }

    }

    ~server() {
        if(sock != -1) {
            close(sock);
        }
    }

private:
    void handle_client(int client_sock) {
        std::cout << client_sock << std::endl;
    }
};



int main(int argc, char** argv) {
    if(argc != 2) {
        std::cout << "Usage: " << argv[0] << " <PORT>" << std::endl;
        return 1;
    }


    return  0;
}

