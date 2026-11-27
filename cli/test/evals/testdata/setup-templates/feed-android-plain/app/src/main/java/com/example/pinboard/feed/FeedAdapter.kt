package com.example.pinboard.feed

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.ImageView
import android.widget.TextView
import androidx.recyclerview.widget.RecyclerView
import coil.load
import com.example.pinboard.R

class FeedAdapter : RecyclerView.Adapter<FeedAdapter.Holder>() {
    private val items = mutableListOf<FeedItem>()

    fun append(page: List<FeedItem>) {
        val start = items.size
        items += page
        notifyItemRangeInserted(start, page.size)
    }

    override fun getItemCount() = items.size

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder =
        Holder(LayoutInflater.from(parent.context).inflate(R.layout.item_post, parent, false))

    override fun onBindViewHolder(holder: Holder, position: Int) = holder.bind(items[position])

    class Holder(view: View) : RecyclerView.ViewHolder(view) {
        private val image: ImageView = view.findViewById(R.id.image)
        private val title: TextView = view.findViewById(R.id.title)
        private val author: TextView = view.findViewById(R.id.author)

        fun bind(item: FeedItem) {
            title.text = item.title
            author.text = item.author
            image.load(item.imageUrl)
        }
    }
}
