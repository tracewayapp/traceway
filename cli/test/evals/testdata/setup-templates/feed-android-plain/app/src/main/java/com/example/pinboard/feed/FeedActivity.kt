package com.example.pinboard.feed

import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.example.pinboard.R
import com.example.pinboard.data.Network
import kotlinx.coroutines.launch

class FeedActivity : AppCompatActivity() {
    private val adapter = FeedAdapter()
    private var cursor: String? = null
    private var loading = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_feed)
        val list = findViewById<RecyclerView>(R.id.feed)
        list.layoutManager = LinearLayoutManager(this)
        list.adapter = adapter
        list.addOnScrollListener(object : RecyclerView.OnScrollListener() {
            override fun onScrolled(rv: RecyclerView, dx: Int, dy: Int) {
                val lm = rv.layoutManager as LinearLayoutManager
                if (lm.findLastVisibleItemPosition() > adapter.itemCount - 5) loadMore()
            }
        })
        loadMore()
    }

    private fun loadMore() {
        if (loading) return
        loading = true
        lifecycleScope.launch {
            try {
                val page = Network.api.feed(cursor)
                cursor = page.nextCursor
                adapter.append(page.items)
            } catch (e: Exception) {
                Toast.makeText(this@FeedActivity, R.string.feed_error, Toast.LENGTH_SHORT).show()
            } finally {
                loading = false
            }
        }
    }
}
