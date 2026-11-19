package com.example.pinboard.media

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Bundle
import android.widget.ImageView
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.example.pinboard.PinboardApp
import com.example.pinboard.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.Request

/**
 * Shows one post's image at full resolution, pinch-to-zoom.
 */
class FullImageActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_POST = "post_id"
    }

    private lateinit var image: ImageView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_full_image)
        image = findViewById(R.id.full_image)
        val postId = intent.getStringExtra(EXTRA_POST) ?: return finish()
        lifecycleScope.launch {
            val url = PinboardApp.api.post(postId).image?.url ?: return@launch finish()
            val bitmap = withContext(Dispatchers.IO) { download(url) }
            image.setImageBitmap(bitmap)
        }
    }

    private fun download(url: String): Bitmap {
        val request = Request.Builder().url(url).build()
        PinboardApp.http.newCall(request).execute().use { response ->
            val body = response.body ?: error("empty body for $url")
            return decodeOriginal(body.byteStream())
        }
    }

    // Originals are up to 6000x4000; decoded at full size so zoom stays sharp.
    private fun decodeOriginal(stream: java.io.InputStream): Bitmap {
        val options = BitmapFactory.Options().apply {
            inPreferredConfig = Bitmap.Config.ARGB_8888
            inMutable = false
        }
        return BitmapFactory.decodeStream(stream, null, options) ?: error("undecodable image")
    }
}
