// Nib macOS 平台层：NSWindow + WKWebView 的最小封装。
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>
#include <string.h>

// Go 侧导出（webview_darwin.go 中 //export）
extern void nibIPCMessage(void* goHandle, char* body);
extern void nibDriverMessage(void* goHandle, char* body);
extern void nibScreenshotResult(void* goHandle, unsigned long long id, void* data, int size, const char* error);

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
    "var timer=setTimeout(function(){delete window.__nibPending[id];reject({code:'ipc/timeout',message:'binding timed out'});},30000);window.__nibPending[id]={resolve:resolve,reject:reject,timer:timer};"
    "window.webkit.messageHandlers.nib.postMessage(JSON.stringify({id:id,dir:'request',call:call,args:args===undefined?null:args}));"
    "});};"
    "window.__nibResolve=function(id,ok,b64){"
    "var p=window.__nibPending[id];if(!p)return;"
    "clearTimeout(p.timer);delete window.__nibPending[id];"
    "var env=JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(b64),function(c){return c.charCodeAt(0);})));"
    "if(ok){p.resolve(env.result);}else{p.reject(env.error||env);}"
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
    if ([msg.name isEqualToString:@"nibDriver"]) nibDriverMessage(self.nib->goHandle, strdup([msg.body UTF8String]));
    else nibIPCMessage(self.nib->goHandle, strdup([msg.body UTF8String]));
}
@end

@interface NibWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation NibWindowDelegate
- (void)windowWillClose:(NSNotification*)notification {
    (void)notification;
    [NSApp stop:nil];
    NSEvent* e = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:e atStart:NO];
}
@end

#pragma mark - C 接口实现

void* nibWindowCreate(const char* title, int width, int height, int devtools, int hidden) {
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
    [[config userContentController] addScriptMessageHandler:handler name:@"nibDriver"];
    WKUserScript* script = [[WKUserScript alloc] initWithSource:kBridgeJS
                                                  injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                               forMainFrameOnly:YES];
    [[config userContentController] addUserScript:script];

    nib->webview = [[WKWebView alloc] initWithFrame:frame configuration:config];
    [nib->window setContentView:nib->webview];
    if (!hidden) {
        [nib->window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    }

    return nib;
}

void nibWindowSetGoHandle(void* w, void* handle) {
    ((NibWindow*)w)->goHandle = handle;
}

// 说明：AppKit/WKWebView 只允许在主线程操作。
// Go 侧任何 goroutine 都可能调到这里（如 harness shell 的转发 goroutine），
// 因此除 create/run（约定主线程调用）外，全部异步派发到主队列执行。

void nibWindowNavigate(void* w, const char* target) {
    NibWindow* nib = (NibWindow*)w;
    NSString* t = [NSString stringWithUTF8String:target];
    dispatch_async(dispatch_get_main_queue(), ^{
        if ([t hasPrefix:@"<"]) {
            [nib->webview loadHTMLString:t baseURL:nil];
        } else {
            NSURL* url = [NSURL URLWithString:t];
            [nib->webview loadRequest:[NSURLRequest requestWithURL:url]];
        }
    });
}

void nibWindowEval(void* w, const char* js) {
    NibWindow* nib = (NibWindow*)w;
    NSString* script = [NSString stringWithUTF8String:js];
    dispatch_async(dispatch_get_main_queue(), ^{
        [nib->webview evaluateJavaScript:script completionHandler:nil];
    });
}

void nibWindowSetTitle(void* w, const char* title) {
    NibWindow* nib = (NibWindow*)w;
    NSString* t = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        [nib->window setTitle:t];
    });
}

void nibWindowResize(void* w, int width, int height) {
    NibWindow* nib = (NibWindow*)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSRect frame = [nib->window frame];
        frame.size = NSMakeSize(width, height);
        [nib->window setFrame:frame display:YES];
    });
}

void nibWindowRun(void* w) {
    (void)w;
    [NSApp run]; // 必须在主线程：Go 侧 webview.New 已 runtime.LockOSThread
}

void nibWindowClose(void* w) {
    NibWindow* nib = (NibWindow*)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [nib->window close];
    });
}

void nibWindowScreenshot(void* w, unsigned long long id) {
 NibWindow* nib=(NibWindow*)w;
 dispatch_async(dispatch_get_main_queue(), ^{
  WKSnapshotConfiguration* config=[[WKSnapshotConfiguration alloc] init];
  [nib->webview takeSnapshotWithConfiguration:config completionHandler:^(NSImage* image, NSError* error){
   if(error || !image){nibScreenshotResult(nib->goHandle,id,NULL,0,[[error localizedDescription] UTF8String] ?: "snapshot unavailable");return;}
   NSBitmapImageRep* rep=[NSBitmapImageRep imageRepWithData:[image TIFFRepresentation]];
   NSData* png=[rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
   nibScreenshotResult(nib->goHandle,id,(void*)[png bytes],(int)[png length],png ? NULL : "PNG encoding failed");
  }];
 });
}
