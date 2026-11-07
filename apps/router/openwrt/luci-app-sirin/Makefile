include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-sirin
PKG_VERSION:=1.4.0
PKG_RELEASE:=1

PKG_MAINTAINER:=Sirin VPN Team <support@sirin.io>
PKG_LICENSE:=MIT

LUCI_TITLE:=LuCI support for Sirin VPN
LUCI_DEPENDS:=+luci-base +rpcd +sirin-client
LUCI_PKGARCH:=all

include $(TOPDIR)/feeds/luci/luci.mk

# Fallback if building as standalone package without feeds/luci:
ifeq ($(wildcard $(TOPDIR)/feeds/luci/luci.mk),)
include $(INCLUDE_DIR)/package.mk

define Package/luci-app-sirin
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=LuCI support for Sirin VPN
  PKGARCH:=all
  DEPENDS:=+luci-base +rpcd +sirin-client
endef

define Build/Compile
endef

define Package/luci-app-sirin/install
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./root/usr/share/luci/menu.d/luci-app-sirin.json $(1)/usr/share/luci/menu.d/
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./root/usr/share/rpcd/acl.d/luci-app-sirin.json $(1)/usr/share/rpcd/acl.d/
	$(INSTALL_DIR) $(1)/usr/libexec/rpcd
	$(INSTALL_BIN) ./root/usr/libexec/rpcd/sirin $(1)/usr/libexec/rpcd/sirin
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/sirin
	$(INSTALL_DATA) ./htdocs/luci-static/resources/view/sirin/overview.js $(1)/www/luci-static/resources/view/sirin/
endef

define Package/luci-app-sirin/postinst
#!/bin/sh
[ -n "$$IPKG_INSTROOT" ] || {
	rm -f /tmp/luci-indexcache /tmp/luci-modulecache/* 2>/dev/null || true
	/etc/init.d/rpcd reload 2>/dev/null || true
}
exit 0
endef

$(eval $(call BuildPackage,luci-app-sirin))
endif
