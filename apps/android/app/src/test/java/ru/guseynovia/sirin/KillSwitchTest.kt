package ru.guseynovia.sirin

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class KillSwitchTest {

    @Test
    fun testKillSwitchDefaultEnabled() {
        // По умолчанию Kill Switch всегда включён ради безопасности пользователя
        KillSwitch.enabled.value = true
        assertTrue(KillSwitch.enabled.value)
    }

    @Test
    fun testKillSwitchToggle() {
        KillSwitch.enabled.value = false
        assertEquals(false, KillSwitch.enabled.value)
        KillSwitch.enabled.value = true
        assertEquals(true, KillSwitch.enabled.value)
    }
}
