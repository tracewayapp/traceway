package com.example.pinboard.feed

import android.content.Intent
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.example.pinboard.PinboardApp
import com.example.pinboard.R
import com.example.pinboard.media.FullImageActivity
import kotlinx.coroutines.launch

class FeedActivity : AppCompatActivity() {
    private val adapter = FeedAdapter(onOpen = ::open)
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
                val page = PinboardApp.api.feed(cursor)
                cursor = page.nextCursor
                adapter.submitList(adapter.currentList + page.items)
                println("feed page loaded: ${page.items.size} items")
            } finally {
                loading = false
            }
        }
    }

    private fun open(item: FeedItem) {
        startActivity(Intent(this, FullImageActivity::class.java).putExtra(FullImageActivity.EXTRA_POST, item.id))
    }
}
