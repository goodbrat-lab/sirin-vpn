include $(TOPDIR)/rules.mk

PKG_NAME:=sirin-client
PKG_VERSION:=1.4.0
PKG_RELEASE:=1

PKG_MAINTAINER:=Sirin VPN Team <support@sirin.io>
PKG_LICENSE:=MIT

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/sirin-client
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=VPN
  TITLE:=Sirin VPN Router Client (TUN + RuDirect + XHTTP/VLESS)
  URL:=https://github.com/goodbrat-lab/sirin-vpn
  DEPENDS:=+ca-bundle +kmod-tun +nftables +ip-full
endef

define Package/sirin-client/description
  Sirin VPN client daemon for OpenWrt routers.
  Supports full tunneling, RuDirect RIPE NCC CIDR bypass,
  port-based split tunneling and anti-censorship protocols.
endef

define Package/sirin-client/conffiles
/etc/sirin/client.toml
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./src/* $(PKG_BUILD_DIR)/ 2>/dev/null || true
endef

define Build/Compile
	( \
		cd $(PKG_BUILD_DIR) && \
		GOOS=linux \
		CGO_ENABLED=0 \
		go build -ldflags="-s -w -X main.Version=$(PKG_VERSION)" -o $(PKG_BUILD_DIR)/sirin-client . \
	)
endef

define Package/sirin-client/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/sirin-client $(1)/usr/bin/sirin-client
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/sirin.init $(1)/etc/init.d/sirin
	$(INSTALL_DIR) $(1)/etc/sirin
	$(INSTALL_DATA) ./files/client.toml.example $(1)/etc/sirin/client.toml.example
endef

define Package/sirin-client/postinst
#!/bin/sh
[ -n "$$IPKG_INSTROOT" ] || {
	if [ ! -f /etc/sirin/client.toml ] && [ -f /etc/sirin/client.toml.example ]; then
		cp /etc/sirin/client.toml.example /etc/sirin/client.toml
	fi
	/etc/init.d/sirin enable
}
exit 0
endef

define Package/sirin-client/prerm
#!/bin/sh
[ -n "$$IPKG_INSTROOT" ] || {
	/etc/init.d/sirin stop 2>/dev/null || true
	/etc/init.d/sirin disable 2>/dev/null || true
}
exit 0
endef

$(eval $(call BuildPackage,sirin-client))
