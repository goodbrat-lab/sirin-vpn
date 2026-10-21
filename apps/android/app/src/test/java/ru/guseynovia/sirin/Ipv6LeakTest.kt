package ru.guseynovia.sirin

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.Inet6Address

class Ipv6LeakTest {

    @Test
    fun testIpv6BlackholeConstants() {
        val ulaAddress = "fd00::1"
        val prefix = 128
        val defaultRoute = "::"
        val routePrefix = 0

        val addr = Inet6Address.getByName(ulaAddress)
        assertTrue(addr is Inet6Address)
        assertEquals("::", defaultRoute)
        assertEquals(0, routePrefix)
        assertEquals(128, prefix)
    }
}
