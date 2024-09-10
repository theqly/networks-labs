#include <arpa/inet.h>
#include <chrono>
#include <iostream>
#include <sys/socket.h>
#include <sys/poll.h>
#include <map>
#include <thread>
#include <unistd.h>

#define PORT 1234
#define BUFFER_SIZE 512
#define WAIT_TIME 2500
#define ALIVE_TIME 1000

void send_multicast(const int sock, const sockaddr_in group_addr){
    std::string message = std::to_string(getpid());
    if(sendto(sock, message.c_str(), message.size(), 0, (sockaddr*)&group_addr, sizeof(group_addr)) == -1){
        perror("error in sendto()");
    }
}

void receive_multicast(const int sock, struct pollfd* pfds, std::map<std::string, std::chrono::time_point<std::chrono::steady_clock>>& alive){
    char buffer[BUFFER_SIZE];
    sockaddr_in sender_addr{};
    socklen_t sender_addr_len = sizeof(sender_addr);

    auto before_poll = std::chrono::steady_clock::now();
    int num_events = poll(pfds, 1, WAIT_TIME);

    if(num_events == 0) {
        return;
    }

    ssize_t recv_len = recvfrom(sock, buffer, BUFFER_SIZE, 0, (sockaddr*)&sender_addr, &sender_addr_len);

    if(recv_len > 0){
        buffer[recv_len] = '\0';
        std::string sender = inet_ntoa(sender_addr.sin_addr);
        alive[sender + " " + buffer] = std::chrono::steady_clock::now();
    } else if(recv_len == -1){
        perror("error in recvfrom()");
    }
    auto timeout = std::chrono::duration_cast<std::chrono::milliseconds >(before_poll - std::chrono::steady_clock::now());
    if(timeout.count() < WAIT_TIME) std::this_thread::sleep_for(std::chrono::milliseconds(WAIT_TIME - timeout.count()));
}

void cleanup(std::map<std::string, std::chrono::time_point<std::chrono::steady_clock>>& alive){
    auto now = std::chrono::steady_clock::now();
    for (auto it = alive.begin(); it != alive.end(); ) {
        auto timeout = std::chrono::duration_cast<std::chrono::seconds>(now - it->second);
        if (timeout.count() >= ALIVE_TIME) {
            it = alive.erase(it);
        } else {
            ++it;
        }
    }
}

void print_alive(std::map<std::string, std::chrono::time_point<std::chrono::steady_clock>>& alive){
    if(!alive.empty()) std::cout << "current programs: " << std::endl;

    for(const auto& it : alive){
        std::cout << it.first << std::endl;
    }
}

int main(int argc, char* argv[]) {
    if(argc != 2){
        std::cerr << "Usage: " << argv[0] << " <Multicast address>" << std::endl;
        return 1;
    }

    std::string multicast_address = argv[1];

    int recv_sock = socket(AF_INET, SOCK_DGRAM, 0);
    if(recv_sock < 0){
        perror("error in socket()");
        return 1;
    }

    int opt = 1;
    if (setsockopt(recv_sock, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
        perror("error in setsockopt()");
        return 1;
    }

    sockaddr_in local_addr{};
    local_addr.sin_family = AF_INET;
    local_addr.sin_port = htons(PORT);
    local_addr.sin_addr.s_addr = htonl(INADDR_ANY);


    if (bind(recv_sock, (sockaddr*)&local_addr, sizeof(local_addr)) < 0) {
        perror("error in bind()");
        return 1;
    }

    int send_sock = socket(AF_INET, SOCK_DGRAM, 0);
    if(send_sock < 0){
        perror("error in socket()");
        return 1;
    }

    sockaddr_in group_addr{};
    group_addr.sin_family = AF_INET;
    group_addr.sin_port = htons(PORT);

    if(inet_pton(AF_INET, multicast_address.c_str(), &group_addr.sin_addr) <= 0){
        std::cerr << "invalid multicast address" << std::endl;
        return 1;
    }

    ip_mreq mreq{};
    mreq.imr_multiaddr = group_addr.sin_addr;
    mreq.imr_interface.s_addr = htonl(INADDR_ANY);

    if(setsockopt(recv_sock, IPPROTO_IP, IP_ADD_MEMBERSHIP, &mreq, sizeof(mreq))){
        perror("error in setsockopt()");
        return 1;
    }

    struct pollfd pfds[1];
    pfds[0].fd = recv_sock;
    pfds[0].events = POLLIN;

    std::map<std::string, std::chrono::time_point<std::chrono::steady_clock>> alive;

    while(true){
        receive_multicast(recv_sock,pfds, alive);

        send_multicast(send_sock, group_addr);

        print_alive(alive);

        cleanup(alive);
    }

    return 0;
}
