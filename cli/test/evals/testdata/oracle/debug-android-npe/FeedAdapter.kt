package com.example.pinboard.feed

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.ImageView
import android.widget.TextView
import androidx.constraintlayout.widget.ConstraintLayout
import androidx.core.view.updateLayoutParams
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView
import coil.load
import coil.transform.CircleCropTransformation
import com.example.pinboard.R

/**
 * Binds feed posts. Each card shows the author, the text and, since 5.8.0,
 * the image at its own aspect ratio instead of a fixed square crop.
 */
class FeedAdapter(
    private val onOpen: (FeedItem) -> Unit,
) : ListAdapter<FeedItem, FeedAdapter.PostViewHolder>(DIFF) {

    class PostViewHolder(view: View) : RecyclerView.ViewHolder(view) {
        val avatar: ImageView = view.findViewById(R.id.avatar)
        val author: TextView = view.findViewById(R.id.author)
        val text: TextView = view.findViewById(R.id.text)
        val thumbnail: ImageView = view.findViewById(R.id.thumbnail)
        val likes: TextView = view.findViewById(R.id.likes)
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): PostViewHolder {
        val view = LayoutInflater.from(parent.context).inflate(R.layout.item_post, parent, false)
        return PostViewHolder(view)
    }

    override fun onBindViewHolder(holder: PostViewHolder, position: Int) {
        val item = getItem(position)
        holder.author.text = item.author.name
        holder.avatar.load(item.author.avatarUrl) {
            crossfade(true)
            transformations(CircleCropTransformation())
            placeholder(R.drawable.avatar_placeholder)
        }
        holder.text.text = item.text
        holder.likes.text = holder.itemView.resources.getQuantityString(R.plurals.likes, item.likes, item.likes)
        bindImage(holder, item)
        holder.itemView.setOnClickListener { onOpen(item) }
    }

    private fun bindImage(holder: PostViewHolder, item: FeedItem) {
        val image = item.image
        if (image == null) {
            // Text posts have no image.
            holder.thumbnail.visibility = View.GONE
            return
        }
        holder.thumbnail.visibility = View.VISIBLE
        holder.thumbnail.updateLayoutParams<ConstraintLayout.LayoutParams> {
            dimensionRatio = "${image.width}:${image.height}"
        }
        holder.thumbnail.load(image.url) {
            crossfade(true)
            size(holder.thumbnail.width.coerceAtLeast(1), coil.size.Dimension.Undefined)
        }
    }

    private companion object {
        val DIFF = object : DiffUtil.ItemCallback<FeedItem>() {
            override fun areItemsTheSame(a: FeedItem, b: FeedItem) = a.id == b.id
            override fun areContentsTheSame(a: FeedItem, b: FeedItem) = a == b
        }
    }
}
