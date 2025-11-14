// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <sys/epoll.h>
#include <stdatomic.h>

#define MAX_EVENTS 1024

#define STATE_OK 0
#define STATE_REMOTE_CLOSED 1
#define STATE_CLOSED 2

typedef struct {
    void* fd;
    atomic_uint_least64_t state;
} conn_stater_t;

// This function is called from a Go goroutine and will block forever in the epoll loop.
// It will run in the CGO context and automatically release the P when blocking.
// The freeack_ptr is passed from Go to notify when to cleanup pollcache.
int cgo_epoll_wait_loop(int epfd, atomic_int_least32_t* freeack_ptr) {
    struct epoll_event events[MAX_EVENTS];

    while (1) {
        int n = epoll_wait(epfd, events, MAX_EVENTS, -1);
        if (n < 0) {
            if (errno == EINTR) continue;
            return errno;
        }

        for (int i = 0; i < n; i++) {
            struct epoll_event* ev = &events[i];
            // ev->data.ptr points to fd.conn field, which contains the conn_stater address
            atomic_uintptr_t* conn_ptr_field = (atomic_uintptr_t*)ev->data.ptr;

            void* conn_ptr = (void*)atomic_load(conn_ptr_field);
            if (conn_ptr == NULL) continue;

            conn_stater_t* conn = (conn_stater_t*)conn_ptr;

            if (ev->events & (EPOLLHUP | EPOLLRDHUP | EPOLLERR)) {
                atomic_compare_exchange_strong(&conn->state, &(uint64_t){STATE_OK}, STATE_REMOTE_CLOSED);
            }
        }

        // we can make sure that there is no op remaining if finished handling all events
        atomic_store(freeack_ptr, 1);
    }

    return 0;
}