async function update() {
    const errorBox = document.getElementById("errorBox");
    errorBox.hidden = true;

    // Fetch services.
    let services = [];
    try {
        const response = await fetch("api/v1/services");
        if (!response.ok) {
            throw new Error(`${response.statusText}.`);
        }
        services = await response.json();
    } catch (error) {
        const errorMessage = document.getElementById("errorMessage");
        errorMessage.textContent = "Could not load services: " + error.message;
        errorBox.hidden = false;
        return;
    }

    // Sort on name.
    const collator = new Intl.Collator();
    sercies = services.sort((a, b) => collator.compare(a.name, b.name));

    // Build cards.
    const cardTemplate = document.getElementById("cardTemplate");
    const cards = services.map(service => {
        const card = document.importNode(cardTemplate.content, true);

        const name        = card.querySelector(".name");
        const description = card.querySelector(".description");

        name.textContent = service.name;
        name.setAttribute("href", service.link);
        description.textContent = service.description;

        return card;
    });

    // Replace current cards.
    const cardHolder = document.getElementById("cardHolder");
    cardHolder.replaceChildren(...cards);
}

setInterval(update, 60 * 1000);
window.onload = update;
