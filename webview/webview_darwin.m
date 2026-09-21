// Nib macOS 平台层：NSWindow + WKWebView 的最小封装。
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>
#include <string.h>

// Go 侧导出（webview_darwin.go 中 //export）
extern void nibIPCMessage(void* goHandle, char* body);

#pragma mark - 前端桥接脚本

// 注入到每个页面的最前面：window.nib.invoke + 响应分发。
// 响应经 base64 传递，避免 JSON 转义问题。
static NSString* const kBridgeJS =
    @"(function(){"
    "window.nib=window.nib||{};"
    "window.__nibPending={};"
    "var seq=0;"
    "window.nib.invoke=function(call,args){"
    "return new Promise(function(resolve,reject){"
    "var id='m'+(++seq);"
    "window.__nibPending[id]={resolve:resolve,reject:reject};"
    "window.webkit.messageHandlers.nib.postMessage(JSON.stringify({id:id,call:call,args:args||null}));"
    "});};"
    "window.__nibResolve=function(id,ok,b64){"
    "var p=window.__nibPending[id];if(!p)return;"
    "delete window.__nibPending[id];"
    "var payload=JSON.parse(atob(b64));"
    "if(ok)p.resolve(payload);else p.reject(payload);"
    "};"
    "})();";

#pragma mark - 对象定义

typedef struct {
    NSWindow* window;
    WKWebView* webview;
    void* goHandle;
} NibWindow;

@interface NibScriptHandler : NSObject <WKScriptMessageHandler>
@property (nonatomic, assign) NibWindow* nib;
@end

@implementation NibScriptHandler
- (void)userContentController:(WKUserContentController*)ucc
      didReceiveScriptMessage:(WKScriptMessage*)msg {
    (void)ucc;
    if (![msg.body isKindOfClass:[NSString class]] || self.nib == NULL) return;
    nibIPCMessage(self.nib->goHandle, strdup([msg.body UTF8String]));
}
@end

@interface NibWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation NibWindowDelegate
- (void)windowWillClose:(NSNotification*)notification {
    (void)notification;
    [NSApp terminate:nil];
}
@end

#pragma mark - C 接口实现

void* nibWindowCreate(const char* title, int width, int height, int devtools) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

    NibWindow* nib = (NibWindow*)calloc(1, sizeof(NibWindow));

    NSRect frame = NSMakeRect(0, 0, width, height);
    NSWindowStyleMask mask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                         | NSWindowStyleMaskResizable | NSWindowStyleMaskMiniaturizable;
    nib->window = [[NSWindow alloc] initWithContentRect:frame
                                              styleMask:mask
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    [nib->window setTitle:[NSString stringWithUTF8String:title]];
    [nib->window setReleasedWhenClosed:NO];
    [nib->window setDelegate:[[NibWindowDelegate alloc] init]];
    [nib->window center];

    WKWebViewConfiguration* config = [[WKWebViewConfiguration alloc] init];
    if (devtools) {
        [config.preferences setValue:@YES forKey:@"developerExtrasEnabled"];
    }
    NibScriptHandler* handler = [[NibScriptHandler alloc] init];
    handler.nib = nib;
    [[config userContentController] addScriptMessageHandler:handler name:@"nib"];
    WKUserScript* script = [[WKUserScript alloc] initWithSource:kBridgeJS
                                                  injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                               forMainFrameOnly:YES];
    [[config userContentController] addUserScript:script];

    nib->webview = [[WKWebView alloc] initWithFrame:frame configuration:config];
    [nib->window setContentView:nib->webview];
    [nib->window makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];

    return nib;
}

void nibWindowSetGoHandle(void* w, void* handle) {
    ((NibWindow*)w)->goHandle = handle;
}

void nibWindowNavigate(void* w, const char* target) {
    NibWindow* nib = (NibWindow*)w;
    NSString* t = [NSString stringWithUTF8String:target];
    if ([t hasPrefix:@"<"]) {
        [nib->webview loadHTMLString:t baseURL:nil];
    } else {
        NSURL* url = [NSURL URLWithString:t];
        [nib->webview loadRequest:[NSURLRequest requestWithURL:url]];
    }
}

void nibWindowEval(void* w, const char* js) {
    NibWindow* nib = (NibWindow*)w;
    NSString* script = [NSString stringWithUTF8String:js];
    [nib->webview evaluateJavaScript:script completionHandler:nil];
}

void nibWindowSetTitle(void* w, const char* title) {
    [((NibWindow*)w)->window setTitle:[NSString stringWithUTF8String:title]];
}

void nibWindowResize(void* w, int width, int height) {
    NibWindow* nib = (NibWindow*)w;
    NSRect frame = [nib->window frame];
    frame.size = NSMakeSize(width, height);
    [nib->window setFrame:frame display:YES];
}

void nibWindowRun(void* w) {
    (void)w;
    [NSApp run];
}

void nibWindowClose(void* w) {
    [((NibWindow*)w)->window close];
}
