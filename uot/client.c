/* @(#)client.c
 */

#include <sys/types.h>          /* See NOTES */
#include <sys/stat.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define BUFSIZE 1500

char *read_from_urandom()
{
	int fd;
	int n;
	char *buf;

	fd = open("/dev/urandom", O_RDONLY);
	buf = malloc(1024);
	n = read(fd, buf, 1024);

	close(fd);
	return buf;
}

int main(int argc, char *argv[])
{
	int fd;
	int n;
	char buf[BUFSIZE];
	char *data;
	struct sockaddr_in remote_addr;
	socklen_t len;

	data = read_from_urandom();
	if (data == NULL) {
		perror("read from urandom error.");
		return -1;
	}

	fd = socket(AF_INET, SOCK_DGRAM, 0);
	if (fd == -1) {
		perror("create socket failed.\n");
		return -1;
	}

	/* init servaddr */
	bzero(&remote_addr, sizeof(remote_addr));
	remote_addr.sin_family = AF_INET;
	remote_addr.sin_port = htons(8899);
	if(inet_pton(AF_INET, argv[1], &remote_addr.sin_addr) <= 0)
	{
		printf("[%s] is not a valid IPaddress\n", argv[1]);
		return -1;
	}

	if(connect(fd, (struct sockaddr *)&remote_addr, sizeof(remote_addr)) == -1)
	{
		perror("connect error");
		return -1;
	}

	write(fd, data, 1024);

	while(1)
	{
		n = read(fd, buf, BUFSIZE);
		n = write(fd, buf, n);
	}

	return 0;
}
