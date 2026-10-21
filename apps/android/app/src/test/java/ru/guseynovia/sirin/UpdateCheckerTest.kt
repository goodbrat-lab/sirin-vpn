package ru.guseynovia.sirin

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class UpdateCheckerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun testIsNewer() {
        assertTrue(UpdateChecker.isNewer("v1.3.1", "1.3.0"))
        assertTrue(UpdateChecker.isNewer("android-v1.3.1", "1.2.0"))
        assertTrue(UpdateChecker.isNewer("2.0.0", "1.9.9"))
        assertFalse(UpdateChecker.isNewer("1.3.0", "1.3.0"))
        assertFalse(UpdateChecker.isNewer("1.2.0", "1.3.0"))
    }

    @Test
    fun testComputeSha256() {
        val testFile = tempFolder.newFile("test.txt")
        testFile.writeText("hello sirin vpn")

        val sha256 = UpdateChecker.computeSha256(testFile)
        assertEquals("a97f619b74350a86284d35de0972813cb4a286fc9048073175d9017c12fe7500", sha256)
    }
}
