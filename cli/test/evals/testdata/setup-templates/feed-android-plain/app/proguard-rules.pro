# Retrofit reads the service interface's generic signatures at runtime.
-keepattributes Signature, InnerClasses, EnclosingMethod
-keepattributes RuntimeVisibleAnnotations, RuntimeVisibleParameterAnnotations
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response

# Gson maps the feed JSON onto these by field name.
-keep class com.example.pinboard.feed.FeedItem { *; }
-keep class com.example.pinboard.data.FeedPage { *; }
