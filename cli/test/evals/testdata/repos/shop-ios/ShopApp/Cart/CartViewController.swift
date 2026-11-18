import UIKit

/// The cart screen (UIKit, a collection view with batch updates).
final class CartViewController: UICollectionViewController {
    private var items: [CartItem] = []
    private let cart: CartService

    init(cart: CartService = .live) {
        self.cart = cart
        super.init(collectionViewLayout: CartViewController.layout())
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

    override func viewDidLoad() {
        super.viewDidLoad()
        collectionView.register(CartItemCell.self, forCellWithReuseIdentifier: CartItemCell.reuseID)
        Task { await reload() }
    }

    override func collectionView(_ collectionView: UICollectionView, numberOfItemsInSection section: Int) -> Int {
        items.count
    }

    override func collectionView(_ collectionView: UICollectionView, cellForItemAt indexPath: IndexPath) -> UICollectionViewCell {
        let cell = collectionView.dequeueReusableCell(withReuseIdentifier: CartItemCell.reuseID, for: indexPath) as! CartItemCell
        cell.configure(with: items[indexPath.item]) { [weak self] in self?.removeItem(at: indexPath) }
        return cell
    }

    private func reload() async {
        items = (try? await cart.items()) ?? []
        collectionView.reloadData()
    }

    private func removeItem(at indexPath: IndexPath) {
        let item = items[indexPath.item]
        Task {
            guard (try? await cart.remove(item.id)) != nil else { return }
            apply(.delete(indexPath))
        }
    }

    private enum Change {
        case delete(IndexPath)
    }

    private func apply(_ change: Change) {
        switch change {
        case .delete(let indexPath):
            collectionView.performBatchUpdates {
                items.remove(at: indexPath.item)
                collectionView.deleteItems(at: [indexPath])
            }
        }
    }

    private static func layout() -> UICollectionViewLayout {
        UICollectionViewCompositionalLayout.list(using: .init(appearance: .plain))
    }
}
