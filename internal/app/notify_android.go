//go:build android

package app

/*
#include <jni.h>
#include <stdlib.h>

// Cached JVM + WailsBridge class/method handles, filled once by JNI_OnLoad
// when System.loadLibrary("wails") runs (the load happens inside
// WailsBridge's static initializer, so the class is guaranteed findable
// there). NOTE: the wails module does not define JNI_OnLoad; if it ever
// starts to, this prologue must merge with it (duplicate symbol at link).
static JavaVM* g_mail_jvm = NULL;
static jclass g_mail_bridge_cls = NULL;
static jfieldID g_mail_instance_fid = NULL;
static jmethodID g_mail_notify_mid = NULL;

__attribute__((visibility("default"))) jint JNI_OnLoad(JavaVM* vm, void* reserved) {
	g_mail_jvm = vm;
	JNIEnv* env = NULL;
	if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION_1_6) != JNI_OK || env == NULL) {
		return JNI_VERSION_1_6;
	}
	jclass cls = (*env)->FindClass(env, "com/wails/app/WailsBridge");
	if (cls == NULL) {
		return JNI_VERSION_1_6;
	}
	g_mail_bridge_cls = (jclass)(*env)->NewGlobalRef(env, cls);
	g_mail_instance_fid = (*env)->GetStaticFieldID(env, cls, "instance",
		"Lcom/wails/app/WailsBridge;");
	g_mail_notify_mid = (*env)->GetMethodID(env, cls, "notifyNewMail",
		"(Ljava/lang/String;Ljava/lang/String;)V");
	return JNI_VERSION_1_6;
}

static void mailNotifyNative(const char* title, const char* text) {
	if (g_mail_jvm == NULL || g_mail_bridge_cls == NULL ||
		g_mail_notify_mid == NULL || g_mail_instance_fid == NULL) {
		return;
	}
	JNIEnv* env = NULL;
	if ((*g_mail_jvm)->AttachCurrentThread(g_mail_jvm, &env, NULL) != JNI_OK || env == NULL) {
		return;
	}
	jobject bridge = (*env)->GetStaticObjectField(env, g_mail_bridge_cls, g_mail_instance_fid);
	if (bridge == NULL) {
		return;
	}
	jstring jtitle = (*env)->NewStringUTF(env, title);
	jstring jtext = (*env)->NewStringUTF(env, text);
	if (jtitle != NULL && jtext != NULL) {
		(*env)->CallVoidMethod(env, bridge, g_mail_notify_mid, jtitle, jtext);
	}
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
	}
	if (jtitle != NULL) (*env)->DeleteLocalRef(env, jtitle);
	if (jtext != NULL) (*env)->DeleteLocalRef(env, jtext);
}
*/
import "C"

import "unsafe"

// platformNotify posts the Android "new mail" notification by calling
// WailsBridge.notifyNewMail over JNI. Safe from any goroutine: the first
// call on a Go thread attaches it to the JVM (idempotent, left attached —
// the poll goroutines are long-lived).
func platformNotify(title, text string) {
	ct := C.CString(title)
	defer C.free(unsafe.Pointer(ct))
	cx := C.CString(text)
	defer C.free(unsafe.Pointer(cx))
	C.mailNotifyNative(ct, cx)
}
