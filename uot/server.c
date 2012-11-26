/* @(#)server.c
 */

#include <sys/types.h>          /* See NOTES */
#include <sys/socket.h>
#include <netinet/in.h>
#include <string.h>

#define BUFSIZE 1500

int main(int argc, char *argv[])
{
	int fd;
	int n;
	char buf[BUFSIZE];
	struct sockaddr_in local_addr;
	struct sockaddr_in remote_addr;
	socklen_t len;

	fd = socket(AF_INET, SOCK_DGRAM, 0);
	if (fd == -1) {
		perror("create socket failed.\n");
		return -1;
	}

	/* init servaddr */
	bzero(&local_addr, sizeof(local_addr));
	local_addr.sin_family = AF_INET;
	local_addr.sin_addr.s_addr = htonl(INADDR_ANY);
	local_addr.sin_port = htons(8899);

	if(bind(fd, (struct sockaddr *)&local_addr, sizeof(local_addr)) == -1)
	{
		perror("bind error.\n");
		return -1;
	}

	while(1)
	{
		len = sizeof(remote_addr);
		n = recvfrom(fd, buf, BUFSIZE, 0, (struct sockaddr*)&remote_addr, &len);
		sendto(fd, buf, n, 0, (struct sockaddr*)&remote_addr, len);
	}

	return 0;
}
