package capture

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreMedia -framework CoreVideo -framework CoreGraphics -framework CoreFoundation -framework AppKit

#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <AppKit/AppKit.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ============================================================
// 全局状态 — 通过 pthread mutex 保证线程安全。
// 单实例设计（匹配系统单显示器模型）。
// ============================================================

// 通过 sck_list_windows 返回给 Go 的窗口信息。
typedef struct {
    uint32_t windowID;
    int      x, y, width, height;
    char     title[256];
    char     appName[256];
    char     bundleID[256];
    int      onScreen;
    int      layer;
} SCKWindowInfo;

typedef struct {
    // 显示器元数据（由 sck_discover 填充）
    int      displayWidth;     // 逻辑点
    int      displayHeight;    // 逻辑点
    double   scaleFactor;      // NSScreen backingScaleFactor
    uint32_t displayID;
    int      refreshRateHz;    // NSScreen maximumFramesPerSecond

    // 最新帧缓冲区（由回调委托填充）
    unsigned char *pixels;
    int      frameWidth;
    int      frameHeight;
    int      frameBytesPerRow;
    uint64_t frameNo;
    volatile int hasNewFrame;

    // 同步锁
    pthread_mutex_t mutex;
} SCKState;

static SCKState        g_sck       = {0};
static SCStream       *g_stream    = nil;
static dispatch_queue_t g_queue    = nil;

// 强引用，防止 ARC 提前释放。
static SCShareableContent *g_content = nil;
static SCDisplay          *g_display = nil;

// ============================================================
// SCStreamOutput 委托 — 接收系统推送的帧。
// 帧数据在锁保护下复制到 g_sck，使 Go 侧
// 可以独立于回调 dispatch queue 读取。
// ============================================================

@interface SCKFrameHandler : NSObject <SCStreamOutput>
@end

@implementation SCKFrameHandler

- (void)stream:(SCStream *)stream
    didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer
                   ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeScreen) return;

    CVImageBufferRef imgBuf = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!imgBuf) return;

    CVPixelBufferLockBaseAddress(imgBuf, kCVPixelBufferLock_ReadOnly);

    int   width       = (int)CVPixelBufferGetWidth(imgBuf);
    int   height      = (int)CVPixelBufferGetHeight(imgBuf);
    int   bytesPerRow = (int)CVPixelBufferGetBytesPerRow(imgBuf);
    void *baseAddr    = CVPixelBufferGetBaseAddress(imgBuf);
    int   dataSize    = bytesPerRow * height;

    pthread_mutex_lock(&g_sck.mutex);

    // 仅在尺寸变化时重新分配。
    int prevSize = g_sck.frameBytesPerRow * g_sck.frameHeight;
    if (dataSize != prevSize || g_sck.pixels == NULL) {
        free(g_sck.pixels);  // free(NULL) 按 C 标准是安全的
        g_sck.pixels = (unsigned char *)malloc(dataSize);
    }

    if (g_sck.pixels) {
        memcpy(g_sck.pixels, baseAddr, dataSize);
        g_sck.frameWidth       = width;
        g_sck.frameHeight      = height;
        g_sck.frameBytesPerRow = bytesPerRow;
        g_sck.frameNo++;
        g_sck.hasNewFrame = 1;
    }

    pthread_mutex_unlock(&g_sck.mutex);

    CVPixelBufferUnlockBaseAddress(imgBuf, kCVPixelBufferLock_ReadOnly);
}

@end

// 保持强引用，防止 ARC 释放 handler。
static SCKFrameHandler *g_handler = nil;

// ============================================================
// C API — ObjC 异步 → 同步 C 的薄封装层。
// ============================================================

// sck_discover 查找可用显示器并缓存元数据。
// displayIndex 选择显示器（0 = 主屏）。
// 成功返回 0，失败返回负数。
int sck_discover(int displayIndex) {
    @autoreleasepool {
        pthread_mutex_init(&g_sck.mutex, NULL);

        __block SCShareableContent *content = nil;
        __block NSError *error = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);

        [SCShareableContent
            getShareableContentExcludingDesktopWindows:NO
                                 onScreenWindowsOnly:YES
                                   completionHandler:^(SCShareableContent *c, NSError *e) {
            content = c;
            error   = e;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        if (error || !content || content.displays.count == 0) {
            return -1;
        }

        int idx = (displayIndex < (int)content.displays.count) ? displayIndex : 0;
        SCDisplay *display = content.displays[idx];

        g_content = content; // 防止 ARC 释放
        g_display = display;

        g_sck.displayWidth  = (int)display.width;
        g_sck.displayHeight = (int)display.height;
        g_sck.displayID     = display.displayID;

        // 从 NSScreen 获取精确的缩放因子和刷新率。
        g_sck.scaleFactor = 2.0; // Retina 安全默认值
        g_sck.refreshRateHz = 60; // 安全默认值
        NSArray<NSScreen *> *screens = [NSScreen screens];
        for (NSScreen *screen in screens) {
            NSNumber *screenNum = screen.deviceDescription[@"NSScreenNumber"];
            if (screenNum && [screenNum unsignedIntValue] == display.displayID) {
                g_sck.scaleFactor = screen.backingScaleFactor;
                if (@available(macOS 12.0, *)) {
                    g_sck.refreshRateHz = (int)screen.maximumFramesPerSecond;
                }
                break;
            }
        }
        // 回退方案：当 NSScreen 未提供时使用 CGDisplayModeGetRefreshRate
        if (g_sck.refreshRateHz <= 0) {
            CGDisplayModeRef mode = CGDisplayCopyDisplayMode(display.displayID);
            if (mode) {
                double hz = CGDisplayModeGetRefreshRate(mode);
                g_sck.refreshRateHz = (hz > 0) ? (int)hz : 60;
                CGDisplayModeRelease(mode);
            } else {
                g_sck.refreshRateHz = 60; // 外接显示器的硬底线
            }
        }

        return 0;
    }
}

// sck_is_argus_window 检查窗口标题是否匹配 Argus 控制台模式。
static BOOL sck_is_argus_window(NSString *title) {
    if (!title || title.length == 0) return NO;
    // 不区分大小写匹配控制台窗口模式
    NSString *lower = [title lowercaseString];
    return [lower containsString:@"argus"] ||
           [lower containsString:@"localhost:8090"] ||
           [lower containsString:@"localhost:3090"] ||
           [lower containsString:@"127.0.0.1:8090"] ||
           [lower containsString:@"127.0.0.1:3090"];
}

// sck_is_argus_app 检查所属应用是否为 Argus 进程。
// 用于捕获标题可能为空的窗口（如 Wails webview）。
static BOOL sck_is_argus_app(SCRunningApplication *app) {
    if (!app) return NO;
    NSString *bid = app.bundleIdentifier;
    if (bid && [bid isEqualToString:@"com.argus.compound"]) return YES;
    NSString *name = [app.applicationName lowercaseString];
    if (name && [name containsString:@"argus"]) return YES;
    return NO;
}

// sck_start_stream 创建并启动捕获流。
// 必须在 sck_discover() 之后调用。fps ∈ [1, 30]。
//
// 使用 initWithDisplay:includingApplications:exceptingWindows: 构建
// 内容过滤器。比 excludingWindows: 更可靠，后者在 macOS 15+
// 上传入空数组时会静默失败。
int sck_start_stream(int fps, int showCursor) {
    @autoreleasepool {
        if (!g_display) return -1;

        // 收集所有应用并识别需要排除的窗口。
        NSArray<SCRunningApplication *> *allApps = g_content ? g_content.applications : @[];
        NSMutableArray<SCWindow *> *excludedWindows = [NSMutableArray array];

        if (g_content) {
            for (SCWindow *win in g_content.windows) {
                if (sck_is_argus_window(win.title) || sck_is_argus_app(win.owningApplication)) {
                    NSLog(@"[SCK] Auto-excluding window: \"%@\" app=%@ (id=%u)",
                          win.title ?: @"",
                          win.owningApplication.bundleIdentifier ?: @"",
                          win.windowID);
                    [excludedWindows addObject:win];
                }
            }
        }
        if (excludedWindows.count > 0) {
            NSLog(@"[SCK] Auto-excluded %lu dashboard window(s) to prevent mirror recursion",
                  (unsigned long)excludedWindows.count);
        }

        // 使用 includingApplications API — 在 macOS 15+ 上比
        // excludingWindows 传空数组更可靠。
        SCContentFilter *filter =
            [[SCContentFilter alloc] initWithDisplay:g_display
                               includingApplications:allApps
                                    exceptingWindows:excludedWindows];

        SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
        cfg.width  = (size_t)(g_sck.displayWidth  * g_sck.scaleFactor);
        cfg.height = (size_t)(g_sck.displayHeight * g_sck.scaleFactor);
        cfg.minimumFrameInterval = CMTimeMake(1, fps);
        cfg.pixelFormat  = kCVPixelFormatType_32BGRA;
        cfg.showsCursor  = (showCursor != 0);
        cfg.queueDepth   = 3; // 背压：消费者慢时丢弃最旧帧

        g_stream  = [[SCStream alloc] initWithFilter:filter
                                       configuration:cfg
                                            delegate:nil];
        g_handler = [[SCKFrameHandler alloc] init];
        g_queue   = dispatch_queue_create("com.argus.sck.frames",
                                          DISPATCH_QUEUE_SERIAL);

        NSError *addErr = nil;
        BOOL ok = [g_stream addStreamOutput:g_handler
                                       type:SCStreamOutputTypeScreen
                              sampleHandlerQueue:g_queue
                                      error:&addErr];
        if (!ok || addErr) return -2;

        __block NSError *startErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [g_stream startCaptureWithCompletionHandler:^(NSError *e) {
            startErr = e;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        return startErr ? -3 : 0;
    }
}

// sck_stop_stream 停止流并释放 ObjC 资源。
int sck_stop_stream(void) {
    @autoreleasepool {
        if (!g_stream) return 0;

        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [g_stream stopCaptureWithCompletionHandler:^(NSError *e) {
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        g_stream  = nil;
        g_handler = nil;
        g_queue   = nil;

        return 0;
    }
}

// sck_get_frame 将最新帧复制到调用者拥有的缓冲区。
// 有新帧返回 1，否则返回 0。
// 调用者必须通过 sck_free_buffer() 释放 *outPixels。
int sck_get_frame(unsigned char **outPixels,
                  int *outWidth, int *outHeight,
                  int *outBytesPerRow, uint64_t *outFrameNo) {
    pthread_mutex_lock(&g_sck.mutex);

    if (!g_sck.hasNewFrame || !g_sck.pixels) {
        pthread_mutex_unlock(&g_sck.mutex);
        return 0;
    }

    int dataSize = g_sck.frameBytesPerRow * g_sck.frameHeight;
    unsigned char *copy = (unsigned char *)malloc(dataSize);
    if (!copy) {
        pthread_mutex_unlock(&g_sck.mutex);
        return 0;
    }
    memcpy(copy, g_sck.pixels, dataSize);

    *outPixels      = copy;
    *outWidth       = g_sck.frameWidth;
    *outHeight      = g_sck.frameHeight;
    *outBytesPerRow = g_sck.frameBytesPerRow;
    *outFrameNo     = g_sck.frameNo;
    g_sck.hasNewFrame = 0;

    pthread_mutex_unlock(&g_sck.mutex);
    return 1;
}

// 缓存显示器元数据的访问器。
int      sck_display_width(void)        { return g_sck.displayWidth; }
int      sck_display_height(void)       { return g_sck.displayHeight; }
double   sck_scale_factor(void)         { return g_sck.scaleFactor; }
uint32_t sck_display_id(void)           { return g_sck.displayID; }
int      sck_display_refresh_rate(void) { return g_sck.refreshRateHz; }

void sck_free_buffer(unsigned char *buf) { free(buf); }

// ============================================================
// 窗口列表 — 枚举所有屏幕上的窗口。
// ============================================================

int sck_list_windows(SCKWindowInfo **outList, int *outCount) {
    @autoreleasepool {
        if (!g_content) return -1;

        NSArray<SCWindow *> *windows = g_content.windows;
        int count = (int)windows.count;
        SCKWindowInfo *list = (SCKWindowInfo *)calloc(count, sizeof(SCKWindowInfo));
        if (!list) return -2;

        int idx = 0;
        for (SCWindow *w in windows) {
            list[idx].windowID = (uint32_t)w.windowID;
            list[idx].x      = (int)w.frame.origin.x;
            list[idx].y      = (int)w.frame.origin.y;
            list[idx].width  = (int)w.frame.size.width;
            list[idx].height = (int)w.frame.size.height;
            list[idx].onScreen = w.onScreen ? 1 : 0;
            list[idx].layer   = (int)w.windowLayer;

            NSString *title = w.title ?: @"";
            [title getCString:list[idx].title maxLength:256 encoding:NSUTF8StringEncoding];

            SCRunningApplication *app = w.owningApplication;
            if (app) {
                NSString *appName = app.applicationName ?: @"";
                NSString *bundleID = app.bundleIdentifier ?: @"";
                [appName getCString:list[idx].appName maxLength:256 encoding:NSUTF8StringEncoding];
                [bundleID getCString:list[idx].bundleID maxLength:256 encoding:NSUTF8StringEncoding];
            }
            idx++;
        }

        *outList = list;
        *outCount = count;
        return 0;
    }
}

void sck_free_window_list(SCKWindowInfo *list) { free(list); }

// ============================================================
// 窗口排除 — 热更新 SCStream 内容过滤器。
// 使用 SCStream.updateContentFilter 实现零停机变更。
// ============================================================

int sck_update_exclusion(uint32_t *windowIDs, int count) {
    @autoreleasepool {
        if (!g_stream || !g_display) return -1;

        // 在进入异步 block 前复制 ID
        uint32_t *idsCopy = NULL;
        if (count > 0 && windowIDs) {
            idsCopy = (uint32_t *)malloc(sizeof(uint32_t) * count);
            memcpy(idsCopy, windowIDs, sizeof(uint32_t) * count);
        }
        int idCount = count;

        __block int result = 0;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);

        // 在后台队列上执行整个过滤器更新，避免
        // 阻塞 CGO 线程导致 SCK 内部 dispatch 死锁。
        dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
            @autoreleasepool {
                // 1. 刷新内容
                __block SCShareableContent *freshContent = nil;
                __block NSError *fetchErr = nil;
                dispatch_semaphore_t fetchSem = dispatch_semaphore_create(0);
                [SCShareableContent
                    getShareableContentExcludingDesktopWindows:NO
                                         onScreenWindowsOnly:NO
                                           completionHandler:^(SCShareableContent *c, NSError *e) {
                    freshContent = c;
                    fetchErr = e;
                    dispatch_semaphore_signal(fetchSem);
                }];
                dispatch_semaphore_wait(fetchSem, DISPATCH_TIME_FOREVER);

                if (fetchErr || !freshContent) {
                    result = -2;
                    free(idsCopy);
                    dispatch_semaphore_signal(done);
                    return;
                }
                g_content = freshContent;

                // 2. 构建排除列表
                NSMutableArray<SCWindow *> *excludeWindows = [NSMutableArray array];
                if (idsCopy && idCount > 0) {
                    for (SCWindow *w in freshContent.windows) {
                        for (int i = 0; i < idCount; i++) {
                            if ((uint32_t)w.windowID == idsCopy[i]) {
                                [excludeWindows addObject:w];
                                break;
                            }
                        }
                    }
                }
                free(idsCopy);

                // 3. 应用过滤器（includingApplications API — macOS 15+ 上更可靠）
                NSLog(@"[SCK] sck_update_exclusion: matched %lu window(s) for exclusion",
                      (unsigned long)excludeWindows.count);
                SCContentFilter *newFilter =
                    [[SCContentFilter alloc] initWithDisplay:g_display
                                       includingApplications:freshContent.applications
                                            exceptingWindows:excludeWindows];

                __block NSError *updateErr = nil;
                dispatch_semaphore_t updateSem = dispatch_semaphore_create(0);
                [g_stream updateContentFilter:newFilter completionHandler:^(NSError *e) {
                    updateErr = e;
                    dispatch_semaphore_signal(updateSem);
                }];
                dispatch_semaphore_wait(updateSem, DISPATCH_TIME_FOREVER);
                if (updateErr) NSLog(@"[SCK] updateContentFilter error: %@", updateErr);

                result = updateErr ? -3 : 0;
                dispatch_semaphore_signal(done);
            }
        });

        dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
        return result;
    }
}

// sck_exclude_app 排除属于指定 bundle ID 的所有窗口。
int sck_exclude_app(const char *bundleID) {
    @autoreleasepool {
        if (!g_stream || !g_display) return -1;

        // 在异步 block 前复制 bundle ID
        NSString *targetBundle = [NSString stringWithUTF8String:bundleID];

        __block int result = 0;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);

        dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
            @autoreleasepool {
                // 1. 刷新内容
                __block SCShareableContent *freshContent = nil;
                __block NSError *fetchErr = nil;
                dispatch_semaphore_t fetchSem = dispatch_semaphore_create(0);
                [SCShareableContent
                    getShareableContentExcludingDesktopWindows:NO
                                         onScreenWindowsOnly:NO
                                           completionHandler:^(SCShareableContent *c, NSError *e) {
                    freshContent = c;
                    fetchErr = e;
                    dispatch_semaphore_signal(fetchSem);
                }];
                dispatch_semaphore_wait(fetchSem, DISPATCH_TIME_FOREVER);

                if (fetchErr || !freshContent) {
                    result = -2;
                    dispatch_semaphore_signal(done);
                    return;
                }
                g_content = freshContent;

                // 2. 构建排除列表
                NSMutableArray<SCWindow *> *excludeWindows = [NSMutableArray array];
                for (SCWindow *w in freshContent.windows) {
                    if ([w.owningApplication.bundleIdentifier isEqualToString:targetBundle]) {
                        [excludeWindows addObject:w];
                    }
                }

                // 3. 应用过滤器（includingApplications API — macOS 15+ 上更可靠）
                NSLog(@"[SCK] sck_exclude_app: matched %lu window(s) for bundle %@",
                      (unsigned long)excludeWindows.count, targetBundle);
                SCContentFilter *newFilter =
                    [[SCContentFilter alloc] initWithDisplay:g_display
                                       includingApplications:freshContent.applications
                                            exceptingWindows:excludeWindows];

                __block NSError *updateErr = nil;
                dispatch_semaphore_t updateSem = dispatch_semaphore_create(0);
                [g_stream updateContentFilter:newFilter completionHandler:^(NSError *e) {
                    updateErr = e;
                    dispatch_semaphore_signal(updateSem);
                }];
                dispatch_semaphore_wait(updateSem, DISPATCH_TIME_FOREVER);
                if (updateErr) NSLog(@"[SCK] updateContentFilter error: %@", updateErr);

                result = updateErr ? -3 : 0;
                dispatch_semaphore_signal(done);
            }
        });

        dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
        return result;
    }
}

// sck_refresh_content 重新获取可共享内容（在 sck_list_windows
// 之前调用以获取最新窗口列表）。
int sck_refresh_content(void) {
    @autoreleasepool {
        __block SCShareableContent *content = nil;
        __block NSError *error = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [SCShareableContent
            getShareableContentExcludingDesktopWindows:NO
                                 onScreenWindowsOnly:YES
                                   completionHandler:^(SCShareableContent *c, NSError *e) {
            content = c;
            error   = e;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        if (error || !content) return -1;
        g_content = content;
        return 0;
    }
}

void sck_cleanup(void) {
    sck_stop_stream();
    pthread_mutex_lock(&g_sck.mutex);
    free(g_sck.pixels);
    g_sck.pixels = NULL;
    pthread_mutex_unlock(&g_sck.mutex);
    pthread_mutex_destroy(&g_sck.mutex);
    g_display = nil;
    g_content = nil;
}

// sck_has_permission 检查当前是否已授予屏幕录制权限。
// 使用 CGPreflightScreenCaptureAccess (macOS 10.15+)，快速同步检查。
// 已授权返回 1，未授权返回 0。
int sck_has_permission(void) {
    if (@available(macOS 10.15, *)) {
        return CGPreflightScreenCaptureAccess() ? 1 : 0;
    }
    return 1; // 10.15 之前：无 TCC 屏幕捕获权限
}

// sck_request_permission 触发系统权限对话框。
// 已授权返回 1，未授权返回 0。非阻塞（对话框可能稍后出现）。
int sck_request_permission(void) {
    if (@available(macOS 10.15, *)) {
        return CGRequestScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}
*/
import "C"

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// SCKCapturer 使用 Apple ScreenCaptureKit 实现 Capturer 接口。
// 相比 DarwinCapturer (CoreGraphics)，它提供：
//   - 事件驱动的帧交付（流回调而非轮询 CGWindowListCreateImage）
//   - 通过 NSScreen 获取精确缩放因子（无硬编码 2.0）
//   - 窗口级/应用级过滤（未来扩展）
//   - 硬件加速捕获（GPU → CVPixelBuffer，零拷贝）
//   - 通过 queueDepth 实现背压（消费者慢时丢弃最旧帧）
type SCKCapturer struct {
	config CaptureConfig

	latestFrame atomic.Pointer[Frame]
	frameChan   chan *Frame // 旧版共享通道

	subscribers   []chan *Frame
	subscribersMu sync.Mutex

	excludedWindows []uint32 // 当前被排除的窗口 ID

	running  atomic.Bool
	stopChan chan struct{}
	mu       sync.Mutex
}

// NewSCKCapturer 创建基于 ScreenCaptureKit 的屏幕捕获器。
// 需要 macOS 12.3+ 和屏幕录制权限。
//
// 包含重试机制以处理 macOS TCC 竞态条件：
// 用户授予屏幕录制权限后应用重启，
// 但 sck_discover 可能在 TCC 完全传播新状态前就运行。
func NewSCKCapturer(config CaptureConfig) (*SCKCapturer, error) {
	displayIdx := 0
	if config.DisplayID != 0 {
		displayIdx = int(config.DisplayID)
	}

	// 预检查：macOS 当前是否授予了屏幕捕获权限？
	hasPermission := int(C.sck_has_permission()) != 0
	if !hasPermission {
		log.Println("[SCK] Screen Recording permission not granted, requesting...")
		C.sck_request_permission()
		// 给 macOS 一点时间处理请求
		time.Sleep(1 * time.Second)
		hasPermission = int(C.sck_has_permission()) != 0
	}

	// 首次尝试
	ret := C.sck_discover(C.int(displayIdx))
	if ret == 0 {
		return &SCKCapturer{
			config:    config,
			frameChan: make(chan *Frame, 4),
			stopChan:  make(chan struct{}),
		}, nil
	}

	// 如果未授权，不重试 — 立即失败。
	// 避免触发多个 TCC 授权对话框。
	if !hasPermission {
		return nil, fmt.Errorf(
			"ScreenCaptureKit discovery failed (code %d): Screen Recording permission not granted. "+
				"Enable it in System Settings → Privacy & Security → Screen Recording, then restart the app",
			ret,
		)
	}

	// 已授权但 sck_discover 仍然失败 — 这是 TCC
	// 传播竞态。延迟重试等待 TCC 跟上。
	log.Println("[SCK] Permission granted but discovery failed, waiting for TCC propagation...")
	for attempt := 2; attempt <= 3; attempt++ {
		time.Sleep(2 * time.Second)
		ret = C.sck_discover(C.int(displayIdx))
		if ret == 0 {
			log.Printf("[SCK] Discovery succeeded on attempt %d", attempt)
			return &SCKCapturer{
				config:    config,
				frameChan: make(chan *Frame, 4),
				stopChan:  make(chan struct{}),
			}, nil
		}
		log.Printf("[SCK] Discovery attempt %d/3 failed (code %d)", attempt, ret)
	}

	return nil, fmt.Errorf(
		"ScreenCaptureKit discovery failed after retries (code %d). "+
			"Try toggling Screen Recording permission off/on and restart the app",
		ret,
	)
}

// Start 以指定 FPS 使用 ScreenCaptureKit 流开始屏幕捕获。
func (s *SCKCapturer) Start(fps int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return fmt.Errorf("capturer is already running")
	}
	if fps < 1 || fps > 30 {
		return fmt.Errorf("FPS must be between 1 and 30, got %d", fps)
	}

	s.config.FPS = fps

	showCursor := 1
	if !s.config.IncludeCursor {
		showCursor = 0
	}

	ret := C.sck_start_stream(C.int(fps), C.int(showCursor))
	if ret != 0 {
		return fmt.Errorf("ScreenCaptureKit stream start failed (code %d)", ret)
	}

	s.running.Store(true)
	go s.frameLoop()
	go s.autoExcludeLoop()
	return nil
}

// Stop 停止屏幕捕获并释放 ScreenCaptureKit 资源。
func (s *SCKCapturer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running.Load() {
		return nil
	}

	s.running.Store(false)
	close(s.stopChan)
	C.sck_stop_stream()
	return nil
}

// LatestFrame 返回最近捕获的帧。
func (s *SCKCapturer) LatestFrame() *Frame {
	return s.latestFrame.Load()
}

// FrameChan 返回旧版共享通道。
// 已废弃：多消费者场景请使用 Subscribe()。
func (s *SCKCapturer) FrameChan() <-chan *Frame {
	return s.frameChan
}

// Subscribe 为该消费者返回一个新的专用通道。
func (s *SCKCapturer) Subscribe() <-chan *Frame {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	ch := make(chan *Frame, 4)
	s.subscribers = append(s.subscribers, ch)
	return ch
}

// Unsubscribe 移除一个订阅通道并关闭它。
func (s *SCKCapturer) Unsubscribe(ch <-chan *Frame) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// IsRunning 返回捕获器是否正在运行。
func (s *SCKCapturer) IsRunning() bool {
	return s.running.Load()
}

// DisplayInfo 返回捕获显示器的元数据，
// ScaleFactor 和 RefreshRateHz 由 NSScreen 精确推导。
func (s *SCKCapturer) DisplayInfo() DisplayInfo {
	return DisplayInfo{
		ID:            uint32(C.sck_display_id()),
		Width:         int(C.sck_display_width()),
		Height:        int(C.sck_display_height()),
		ScaleFactor:   float64(C.sck_scale_factor()),
		RefreshRateHz: int(C.sck_display_refresh_rate()),
	}
}

// ListWindows 返回 ScreenCaptureKit 中所有屏幕上的窗口。
func (s *SCKCapturer) ListWindows() ([]WindowInfo, error) {
	// 刷新内容以获取最新窗口列表
	ret := C.sck_refresh_content()
	if ret != 0 {
		return nil, fmt.Errorf("sck_refresh_content failed: %d", ret)
	}

	var cWindows *C.SCKWindowInfo
	var count C.int
	ret = C.sck_list_windows(&cWindows, &count)
	if ret != 0 {
		return nil, fmt.Errorf("sck_list_windows failed: %d", ret)
	}
	defer C.sck_free_window_list(cWindows)

	n := int(count)
	windows := make([]WindowInfo, 0, n)
	for i := 0; i < n; i++ {
		w := (*C.SCKWindowInfo)(unsafe.Pointer(uintptr(unsafe.Pointer(cWindows)) + uintptr(i)*unsafe.Sizeof(*cWindows)))
		info := WindowInfo{
			WindowID: uint32(w.windowID),
			Title:    C.GoString(&w.title[0]),
			AppName:  C.GoString(&w.appName[0]),
			BundleID: C.GoString(&w.bundleID[0]),
			X:        int(w.x),
			Y:        int(w.y),
			Width:    int(w.width),
			Height:   int(w.height),
			OnScreen: w.onScreen != 0,
			Layer:    int(w.layer),
		}
		// 过滤零尺寸/离屏窗口
		if info.Width > 0 && info.Height > 0 {
			windows = append(windows, info)
		}
	}
	return windows, nil
}

// SetExcludedWindows 设置从捕获中排除的窗口。
// 热更新 SCStream 内容过滤器，无需重启流。
func (s *SCKCapturer) SetExcludedWindows(windowIDs []uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.excludedWindows = make([]uint32, len(windowIDs))
	copy(s.excludedWindows, windowIDs)

	if len(windowIDs) == 0 {
		ret := C.sck_update_exclusion(nil, 0)
		if ret != 0 {
			return fmt.Errorf("sck_update_exclusion clear failed: %d", ret)
		}
		return nil
	}

	ids := make([]C.uint32_t, len(windowIDs))
	for i, id := range windowIDs {
		ids[i] = C.uint32_t(id)
	}
	ret := C.sck_update_exclusion(&ids[0], C.int(len(ids)))
	if ret != 0 {
		return fmt.Errorf("sck_update_exclusion failed: %d", ret)
	}
	return nil
}

// ExcludeApp 排除属于指定 bundle ID 的所有窗口。
func (s *SCKCapturer) ExcludeApp(bundleID string) error {
	cBundleID := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cBundleID))

	ret := C.sck_exclude_app(cBundleID)
	if ret != 0 {
		return fmt.Errorf("sck_exclude_app failed: %d", ret)
	}
	return nil
}

// GetExcludedWindows 返回当前被排除的窗口 ID。
func (s *SCKCapturer) GetExcludedWindows() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]uint32, len(s.excludedWindows))
	copy(result, s.excludedWindows)
	return result
}

// selfExcludePatterns 是标记窗口属于 Argus 控制台的标题子串。
// 不区分大小写匹配。
var selfExcludePatterns = []string{
	"argus",
	"localhost:8090",
	"localhost:3090",
	"127.0.0.1:8090",
	"127.0.0.1:3090",
}

// selfExcludeBundleIDs 是需要排除的精确 bundle 标识符（不区分大小写）。
var selfExcludeBundleIDs = []string{
	"com.argus.compound",
}

// selfExcludeAppNames 是需要排除的应用名称子串（不区分大小写）。
var selfExcludeAppNames = []string{
	"argus",
}

// shouldExcludeWindow 判断窗口是否属于 Argus 自身。
// 检查标题模式、bundle ID 和应用名称子串。
func (s *SCKCapturer) shouldExcludeWindow(w WindowInfo) bool {
	titleLower := strings.ToLower(w.Title)
	for _, pattern := range selfExcludePatterns {
		if strings.Contains(titleLower, pattern) {
			return true
		}
	}
	bundleLower := strings.ToLower(w.BundleID)
	for _, bid := range selfExcludeBundleIDs {
		if bundleLower == bid {
			return true
		}
	}
	appLower := strings.ToLower(w.AppName)
	for _, name := range selfExcludeAppNames {
		if strings.Contains(appLower, name) {
			return true
		}
	}
	return false
}

// autoExcludeLoop 定期扫描屏幕窗口，自动排除匹配 Argus
// 控制台模式的窗口。防止控制台在流启动后打开时出现无限画中画。
func (s *SCKCapturer) autoExcludeLoop() {
	// 初始延迟：让浏览器窗口先打开。
	time.Sleep(2 * time.Second)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastExcluded []uint32 // 跟踪上一次集合以避免冗余更新

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			if !s.running.Load() {
				return
			}
			s.doAutoExclude(&lastExcluded)
		}
	}
}

// doAutoExclude 扫描窗口，如果排除集变化则更新。
func (s *SCKCapturer) doAutoExclude(lastExcluded *[]uint32) {
	windows, err := s.ListWindows()
	if err != nil {
		return
	}

	var toExclude []uint32
	for _, w := range windows {
		if s.shouldExcludeWindow(w) {
			toExclude = append(toExclude, w.WindowID)
		}
	}

	// 如果排除集未变化则跳过更新。
	if uint32SliceEqual(toExclude, *lastExcluded) {
		return
	}

	if len(toExclude) > 0 {
		log.Printf("[SCK] Auto-exclude: updating %d window(s)", len(toExclude))
		if err := s.SetExcludedWindows(toExclude); err != nil {
			log.Printf("[SCK] Auto-exclude update failed: %v", err)
			return
		}
	} else if len(*lastExcluded) > 0 {
		// 之前有排除，现在没有 — 清除。
		log.Println("[SCK] Auto-exclude: clearing exclusions")
		if err := s.SetExcludedWindows(nil); err != nil {
			log.Printf("[SCK] Auto-exclude clear failed: %v", err)
			return
		}
	}

	*lastExcluded = toExclude
}

// uint32SliceEqual 判断两个 uint32 切片是否包含相同元素。
func uint32SliceEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// frameLoop 轮询 SCK 委托回调交付的新帧。
// SCK 在 dispatch queue 上推送帧；此 goroutine 读取最新
// 缓冲帧并分发给所有 Go 侧订阅者。
func (s *SCKCapturer) frameLoop() {
	// 以 2× 配置 FPS 轮询，最小化 ObjC 回调存帧与 Go 读取之间的延迟。
	pollInterval := time.Second / time.Duration(s.config.FPS*2)
	if pollInterval < 5*time.Millisecond {
		pollInterval = 5 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			frame := s.readFrame()
			if frame == nil {
				continue
			}

			s.latestFrame.Store(frame)

			// 旧版共享通道（非阻塞）
			select {
			case s.frameChan <- frame:
			default:
			}

			// 分发给所有订阅者（非阻塞）
			s.subscribersMu.Lock()
			for _, sub := range s.subscribers {
				select {
				case sub <- frame:
				default: // 慢消费者 — 丢弃帧
				}
			}
			s.subscribersMu.Unlock()
		}
	}
}

// readFrame 从 C 侧缓冲区复制最新帧到 Go Frame。
func (s *SCKCapturer) readFrame() *Frame {
	var cPixels *C.uchar
	var cWidth, cHeight, cBytesPerRow C.int
	var cFrameNo C.uint64_t

	got := C.sck_get_frame(&cPixels, &cWidth, &cHeight, &cBytesPerRow, &cFrameNo)
	if got == 0 || cPixels == nil {
		return nil
	}
	defer C.sck_free_buffer(cPixels)

	width := int(cWidth)
	height := int(cHeight)
	stride := int(cBytesPerRow)
	channels := 4
	bufSize := stride * height

	pixels := make([]byte, bufSize)
	copy(pixels, unsafe.Slice((*byte)(unsafe.Pointer(cPixels)), bufSize))

	return &Frame{
		Width:     width,
		Height:    height,
		Stride:    stride,
		Channels:  channels,
		Pixels:    pixels,
		Timestamp: time.Now(),
		FrameNo:   uint64(cFrameNo),
	}
}
