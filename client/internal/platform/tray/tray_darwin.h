#ifndef JEEMI_TRAY_DARWIN_H
#define JEEMI_TRAY_DARWIN_H

#include <stddef.h>
#include <stdint.h>

enum {
    JEEMI_TRAY_READY = 1,
    JEEMI_TRAY_EXIT,
    JEEMI_TRAY_DOUBLE_CLICK,
    JEEMI_TRAY_MENU
};

void jeemiTrayEvent(uintptr_t handle, int event, uint32_t item);
void jeemi_tray_start(uintptr_t handle);
void jeemi_tray_stop(uintptr_t handle);
void jeemi_tray_set_icon(uintptr_t handle, const void *bytes, size_t length);
void jeemi_tray_set_tooltip(uintptr_t handle, const char *text);
void jeemi_tray_enable_menu(uintptr_t handle);
void jeemi_tray_update_item(uintptr_t handle, uint32_t item, const char *title, const char *tooltip);
void jeemi_tray_add_separator(uintptr_t handle);

#endif
