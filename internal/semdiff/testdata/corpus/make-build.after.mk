CC := clang
all: app
app: main.o
	$(CC) -o app main.o
.PHONY: all
